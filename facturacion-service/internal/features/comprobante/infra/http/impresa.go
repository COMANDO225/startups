package http

import (
	"bytes"
	_ "embed"
	b64 "encoding/base64"
	"encoding/json"
	"html/template"
	"io"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"

	"github.com/gofiber/fiber/v3"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

//go:embed impresa.html
var plantillaImpresa string

var tplImpresa = template.Must(template.New("impresa").Parse(plantillaImpresa))

var nombreTipoDoc = map[domain.TipoDoc]string{
	domain.TipoFactura:     "FACTURA ELECTRONICA",
	domain.TipoBoleta:      "BOLETA DE VENTA ELECTRONICA",
	domain.TipoNotaCredito: "NOTA DE CREDITO ELECTRONICA",
	domain.TipoNotaDebito:  "NOTA DE DEBITO ELECTRONICA",
}

// payloadImpresion es lo que la representacion impresa necesita del documento.
// Se lee del payload original en vez de reparsear el XML firmado.
type payloadImpresion struct {
	MontoLetras string `json:"monto_letras"`
	Receptor    struct {
		TipoDoc     string `json:"tipo_doc"`
		NumDoc      string `json:"num_doc"`
		RazonSocial string `json:"razon_social"`
		Direccion   string `json:"direccion"`
	} `json:"receptor"`
	Items []struct {
		Codigo         string      `json:"codigo"`
		Descripcion    string      `json:"descripcion"`
		Unidad         string      `json:"unidad"`
		Cantidad       json.Number `json:"cantidad"`
		PrecioUnitario json.Number `json:"precio_unitario"`
		ValorVenta     json.Number `json:"valor_venta"`
	} `json:"items"`
	Totales struct {
		OperGravadas json.Number `json:"oper_gravadas"`
		IGV          json.Number `json:"igv"`
		ImporteTotal json.Number `json:"importe_total"`
	} `json:"totales"`
}

type vistaImpresa struct {
	Emisor      *domain.Tenant
	Comprobante *domain.Comprobante
	Doc         payloadImpresion
	Titulo      string
	CadenaQR    string
	QRDataURI   template.URL
}

// Impresa devuelve la representacion impresa lista para imprimir. El PDF lo
// genera el navegador desde este HTML: no incorporamos un renderizador propio
// porque el unico maduro para PHP, wkhtmltopdf, esta archivado desde 2023 y
// arrastra un SSRF critico (CVE-2022-35583) que no cabe junto a las claves.
func (ctrl *Controller) Impresa(c fiber.Ctx) error {
	vista, err := ctrl.armarVista(c)
	if err != nil {
		return err
	}

	png, err := generarQR(vista.CadenaQR)
	if err != nil {
		return err
	}
	vista.QRDataURI = template.URL("data:image/png;base64," + b64.StdEncoding.EncodeToString(png))

	var buf bytes.Buffer
	if err := tplImpresa.Execute(&buf, vista); err != nil {
		return err
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Send(buf.Bytes())
}

// QRPNG sirve el codigo suelto, para las impresoras termicas que no renderizan
// HTML.
func (ctrl *Controller) QRPNG(c fiber.Ctx) error {
	vista, err := ctrl.armarVista(c)
	if err != nil {
		return err
	}

	png, err := generarQR(vista.CadenaQR)
	if err != nil {
		return err
	}

	c.Set(fiber.HeaderContentType, "image/png")
	return c.Send(png)
}

func (ctrl *Controller) armarVista(c fiber.Ctx) (*vistaImpresa, error) {
	tenant := tenantDe(c)

	comprobante, err := ctrl.obtener.Execute(c.Context(), tenant.ID, c.Params("id"))
	if err != nil {
		return nil, err
	}

	// Sin XML firmado no hay DigestValue, y sin el el QR no es valido.
	if len(comprobante.XML()) == 0 {
		return nil, domainerr.Conflict("El comprobante todavia no esta firmado").
			WithCode("SIN_XML").
			WithSuggestion("La representacion impresa existe recien cuando SUNAT recibe el documento")
	}

	var doc payloadImpresion
	if err := json.Unmarshal(comprobante.Payload(), &doc); err != nil {
		return nil, domain.ErrPayloadInvalido(err.Error())
	}

	cadena, _ := datosQR(comprobante, tenant)

	return &vistaImpresa{
		Emisor:      tenant,
		Comprobante: comprobante,
		Doc:         doc,
		Titulo:      nombreTipoDoc[comprobante.TipoDoc()],
		CadenaQR:    cadena,
	}, nil
}

// datosQR arma la cadena y su hash. Devuelve vacios mientras el comprobante no
// este firmado: el QR no existe antes de la firma.
func datosQR(c *domain.Comprobante, t *domain.Tenant) (cadena, hash string) {
	if len(c.XML()) == 0 {
		return "", ""
	}

	var doc payloadImpresion
	if json.Unmarshal(c.Payload(), &doc) != nil {
		return "", ""
	}

	hash = domain.DigestDeXML(c.XML())

	return domain.QR{
		RUCEmisor:       t.RUC,
		TipoDoc:         c.TipoDoc(),
		Serie:           c.Serie(),
		Correlativo:     c.Correlativo(),
		IGV:             doc.Totales.IGV.String(),
		Total:           c.ImporteTotal(),
		FechaEmision:    c.FechaEmision(),
		TipoDocReceptor: doc.Receptor.TipoDoc,
		NumDocReceptor:  doc.Receptor.NumDoc,
		ValorResumen:    hash,
	}.Cadena(), hash
}

// SUNAT lo exige en negro, con zona de silencio de al menos 1mm. El tamano final
// lo fija el CSS de la plantilla (maximo 2cm de alto).
func generarQR(cadena string) ([]byte, error) {
	qr, err := qrcode.New(cadena)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := standard.NewWithWriter(sinCerrar{&buf},
		standard.WithQRWidth(8),
		standard.WithBorderWidth(2),
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
	)

	if err := qr.Save(w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// El writer cierra lo que le den; el buffer no tiene nada que cerrar.
type sinCerrar struct{ io.Writer }

func (sinCerrar) Close() error { return nil }
