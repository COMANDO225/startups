package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
)

// El motor devuelve XML y CDR en base64; el limite evita que una respuesta
// descontrolada agote la memoria del worker.
const maxRespuesta = 16 << 20

type Client struct {
	baseURL string
	http    *http.Client
	limite  *limitador
}

func NewClient(baseURL string, timeout time.Duration, maxPorEmisor int) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		limite:  nuevoLimitador(maxPorEmisor),
	}
}

type respuesta struct {
	Nombre        string   `json:"nombre"`
	XMLB64        string   `json:"xml_b64"`
	CDRB64        string   `json:"cdr_b64"`
	Estado        string   `json:"estado"`
	Codigo        string   `json:"codigo"`
	Mensaje       string   `json:"mensaje"`
	Ticket        string   `json:"ticket"`
	Observaciones []string `json:"observaciones"`
	Error         string   `json:"error"`
}

func (c *Client) Emitir(ctx context.Context, t *domain.Tenant, payload []byte, tipoDoc domain.TipoDoc, serie string, correlativo int64, fecha time.Time) (*domain.ResultadoEmision, error) {
	comprobante, err := armarComprobante(payload, t, tipoDoc, serie, correlativo, fecha)
	if err != nil {
		return nil, err
	}
	return c.postSunat(ctx, "/emitir", t.RUC, credenciales(t, map[string]any{"comprobante": comprobante}))
}

func (c *Client) ConsultarTicket(ctx context.Context, t *domain.Tenant, ticket string) (*domain.ResultadoEmision, error) {
	return c.postSunat(ctx, "/consultar-ticket", t.RUC, credenciales(t, map[string]any{"ticket": ticket}))
}

// postSunat es la unica puerta hacia SUNAT: todo lo que sale por aqui respeta el
// turno del emisor. Firmar no pasa por aca porque no sale de la maquina.
func (c *Client) postSunat(ctx context.Context, path, ruc string, body any) (*domain.ResultadoEmision, error) {
	liberar, err := c.limite.adquirir(ctx, ruc)
	if err != nil {
		return nil, domain.ErrMotor("no se obtuvo turno para enviar a SUNAT: " + err.Error())
	}
	defer liberar()

	return c.post(ctx, path, body)
}

func (c *Client) post(ctx context.Context, path string, body any) (*domain.ResultadoEmision, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, domain.ErrMotor(err.Error())
	}
	defer resp.Body.Close()

	// Se lee entero para poder incluir el cuerpo en el error: si el motor
	// devuelve algo que no es JSON, "invalid character '<'" no dice nada.
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxRespuesta))
	if err != nil {
		return nil, domain.ErrMotor("no se pudo leer la respuesta: " + err.Error())
	}

	var r respuesta
	if err := json.Unmarshal(cuerpo, &r); err != nil {
		return nil, domain.ErrMotor(fmt.Sprintf("respuesta no es JSON (HTTP %d): %s", resp.StatusCode, recortar(cuerpo)))
	}

	if r.Error != "" {
		return nil, domain.ErrMotor(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, r.Error))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, domain.ErrMotor(fmt.Sprintf("HTTP %d sin detalle: %s", resp.StatusCode, recortar(cuerpo)))
	}

	xml, _ := base64.StdEncoding.DecodeString(r.XMLB64)
	cdr, _ := base64.StdEncoding.DecodeString(r.CDRB64)

	return &domain.ResultadoEmision{
		Nombre:        r.Nombre,
		XML:           xml,
		CDR:           cdr,
		Estado:        r.Estado,
		Codigo:        r.Codigo,
		Mensaje:       r.Mensaje,
		Ticket:        r.Ticket,
		Observaciones: r.Observaciones,
	}, nil
}

func emisor(t *domain.Tenant) map[string]any {
	return map[string]any{
		"ruc":              t.RUC,
		"razon_social":     t.RazonSocial,
		"nombre_comercial": t.NombreComercial,
		"direccion":        t.Direccion,
		"ubigeo":           t.Ubigeo,
		"departamento":     t.Departamento,
		"provincia":        t.Provincia,
		"distrito":         t.Distrito,
	}
}

// recortar deja el error legible en logs sin volcar una respuesta entera.
func recortar(b []byte) string {
	const max = 300
	s := strings.TrimSpace(strings.ReplaceAll(string(b), "\n", " "))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// Firmar genera el XML de la boleta sin enviarlo: viaja en el resumen diario.
func (c *Client) Firmar(ctx context.Context, t *domain.Tenant, payload []byte, tipoDoc domain.TipoDoc, serie string, correlativo int64, fecha time.Time) (*domain.ResultadoEmision, error) {
	comprobante, err := armarComprobante(payload, t, tipoDoc, serie, correlativo, fecha)
	if err != nil {
		return nil, err
	}
	return c.post(ctx, "/firmar", credenciales(t, map[string]any{"comprobante": comprobante}))
}

func (c *Client) EnviarResumen(ctx context.Context, t *domain.Tenant, r *domain.Resumen, detalles []domain.DetalleResumen) (*domain.ResultadoEmision, error) {
	lineas := make([]map[string]any, len(detalles))
	for i, d := range detalles {
		lineas[i] = map[string]any{
			"tipo_doc":       d.TipoDoc,
			"serie_numero":   d.SerieNumero,
			"estado":         int(d.Estado),
			"cliente_tipo":   d.ClienteTipo,
			"cliente_numero": d.ClienteNumero,
			"total":          d.Total,
			"oper_gravadas":  d.OperGravadas,
			"igv":            d.IGV,
		}
	}

	return c.postSunat(ctx, "/resumen", t.RUC, credenciales(t, map[string]any{
		"resumen": map[string]any{
			"correlativo": r.Correlativo(),
			"fecha_ref":   r.FechaRef().Format("2006-01-02"),
			"emisor":      emisor(t),
			"detalles":    lineas,
		},
	}))
}

func (c *Client) EnviarBaja(ctx context.Context, t *domain.Tenant, r *domain.Resumen, detalles []domain.DetalleBaja) (*domain.ResultadoEmision, error) {
	lineas := make([]map[string]any, len(detalles))
	for i, d := range detalles {
		lineas[i] = map[string]any{
			"tipo_doc":    d.TipoDoc,
			"serie":       d.Serie,
			"correlativo": d.Correlativo,
			"motivo":      d.Motivo,
		}
	}

	return c.postSunat(ctx, "/baja", t.RUC, credenciales(t, map[string]any{
		"resumen": map[string]any{
			"correlativo": r.Correlativo(),
			"fecha_ref":   r.FechaRef().Format("2006-01-02"),
			"emisor":      emisor(t),
			"detalles":    lineas,
		},
	}))
}

// El emisor y la numeracion los pone siempre el servidor: lo que venga en el
// payload del cliente se descarta.
func armarComprobante(payload []byte, t *domain.Tenant, tipoDoc domain.TipoDoc, serie string, correlativo int64, fecha time.Time) (map[string]any, error) {
	var comprobante map[string]any
	if err := json.Unmarshal(payload, &comprobante); err != nil {
		return nil, fmt.Errorf("payload invalido: %w", err)
	}

	comprobante["tipo_doc"] = string(tipoDoc)
	comprobante["serie"] = serie
	comprobante["correlativo"] = correlativo
	// Con desfase horario explicito: Greenter renderiza siempre en America/Lima,
	// y un instante UTC se le convierte al dia anterior.
	comprobante["fecha_emision"] = domain.FechaEmisionLima(fecha).Format(time.RFC3339)
	comprobante["emisor"] = emisor(t)

	return comprobante, nil
}

func credenciales(t *domain.Tenant, extra map[string]any) map[string]any {
	body := map[string]any{
		"cert_pem":   t.CertPEM,
		"ruc":        t.RUC,
		"sol_user":   t.SolUser,
		"sol_pass":   t.SolPass,
		"produccion": t.Produccion,
	}
	maps.Copy(body, extra)
	return body
}
