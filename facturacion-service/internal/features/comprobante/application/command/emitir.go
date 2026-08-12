package command

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	"facturacion-service/internal/shared/transaction"
	"facturacion-service/pkg/pgxerr"
	"facturacion-service/pkg/ulid"
)

type EmitirCmd struct {
	TenantID       string
	IdempotencyKey string
	TipoDoc        string
	Serie          string
	Payload        json.RawMessage
	Moneda         string
	ImporteTotal   string
	FechaEmision   time.Time
}

type Emitir struct {
	repo domain.Repositorio
	cola domain.Encolador
	tx   transaction.Transactor
}

func NewEmitir(repo domain.Repositorio, cola domain.Encolador, tx transaction.Transactor) *Emitir {
	return &Emitir{repo: repo, cola: cola, tx: tx}
}

func (uc *Emitir) Execute(ctx context.Context, cmd EmitirCmd) (*domain.Comprobante, error) {
	tipoDoc := domain.TipoDoc(cmd.TipoDoc)

	// Todo lo que se pueda rechazar se rechaza antes de tocar el contador:
	// un correlativo consumido por un documento invalido deja un hueco en la
	// numeracion que SUNAT despues observa.
	if err := validar(tipoDoc, cmd); err != nil {
		return nil, err
	}

	// Un reintento del cliente devuelve el mismo comprobante en vez de quemar
	// otro correlativo.
	if existente, err := uc.repo.PorIdempotencyKey(ctx, cmd.TenantID, cmd.IdempotencyKey); err == nil && existente != nil {
		return existente, nil
	}

	var creado *domain.Comprobante

	err := uc.tx.RunInTx(ctx, func(ctx context.Context) error {
		correlativo, err := uc.repo.SiguienteCorrelativo(ctx, cmd.TenantID, tipoDoc, cmd.Serie)
		if err != nil {
			return err
		}

		c := domain.New(domain.NuevoComprobante{
			ID:             string(ulid.New()),
			TenantID:       cmd.TenantID,
			IdempotencyKey: cmd.IdempotencyKey,
			TipoDoc:        tipoDoc,
			Serie:          cmd.Serie,
			Correlativo:    correlativo,
			Payload:        cmd.Payload,
			Moneda:         cmd.Moneda,
			ImporteTotal:   cmd.ImporteTotal,
			FechaEmision:   cmd.FechaEmision,
		})

		if err := uc.repo.Crear(ctx, c); err != nil {
			return err
		}

		// Encolar dentro de la transaccion: si el commit falla, el job no existe.
		if err := uc.cola.EncolarEmision(ctx, c.ID()); err != nil {
			return err
		}

		creado = c
		return nil
	})

	if err != nil {
		// Dos requests con la misma clave en paralelo: ambos pasan el chequeo
		// inicial y uno choca contra uq_comprobantes_idempotency. El perdedor
		// hace rollback (devolviendo el correlativo) y recupera el ganador, que
		// es lo que el cliente esperaba desde el principio.
		if pgxerr.IsUniqueViolation(err) {
			if existente, e := uc.repo.PorIdempotencyKey(ctx, cmd.TenantID, cmd.IdempotencyKey); e == nil && existente != nil {
				return existente, nil
			}
		}
		return nil, err
	}

	return creado, nil
}

func validar(tipoDoc domain.TipoDoc, cmd EmitirCmd) error {
	if !tipoDoc.Valido() {
		return domain.ErrTipoDocInvalido(cmd.TipoDoc)
	}

	if !tipoDoc.PrefijoSerieValido(cmd.Serie) {
		return domain.ErrSerieIncoherente(cmd.Serie, cmd.TipoDoc)
	}

	if cmd.FechaEmision.After(time.Now().Add(24 * time.Hour)) {
		return domain.ErrFechaFutura()
	}

	return verificarPayload(tipoDoc, cmd)
}

type payloadEmision struct {
	Totales struct {
		ImporteTotal *json.Number `json:"importe_total"`
	} `json:"totales"`
	Receptor struct {
		TipoDoc     string `json:"tipo_doc"`
		NumDoc      string `json:"num_doc"`
		RazonSocial string `json:"razon_social"`
	} `json:"receptor"`
	Items []json.RawMessage `json:"items"`

	CodigoMotivo      string `json:"codigo_motivo"`
	DescripcionMotivo string `json:"descripcion_motivo"`
	Referencia        struct {
		TipoDoc     string `json:"tipo_doc"`
		SerieNumero string `json:"serie_numero"`
	} `json:"referencia"`
}

// Todo lo que SUNAT sabe rechazar y nosotros podemos comprobar se comprueba
// aqui. El motor tambien valida, pero para entonces el correlativo ya se quemo.
func verificarPayload(tipoDoc domain.TipoDoc, cmd EmitirCmd) error {
	var doc payloadEmision
	if err := json.Unmarshal(cmd.Payload, &doc); err != nil {
		return domain.ErrPayloadInvalido(err.Error())
	}

	if doc.Totales.ImporteTotal == nil {
		return domain.ErrPayloadInvalido("falta totales.importe_total")
	}

	// El importe que se guarda debe ser el mismo que viaja a SUNAT: si difieren,
	// el registro interno contradice al documento legal.
	if !mismoImporte(doc.Totales.ImporteTotal.String(), cmd.ImporteTotal) {
		return domain.ErrImporteIncoherente(cmd.ImporteTotal, doc.Totales.ImporteTotal.String())
	}

	if len(doc.Items) == 0 {
		return domain.ErrPayloadInvalido("items no puede estar vacio")
	}

	if err := domain.ValidarReceptor(domain.Receptor{
		TipoDoc:     doc.Receptor.TipoDoc,
		NumDoc:      doc.Receptor.NumDoc,
		RazonSocial: doc.Receptor.RazonSocial,
	}, tipoDoc, cmd.Serie, cmd.ImporteTotal); err != nil {
		return err
	}

	return domain.ValidarNota(domain.Nota{
		CodigoMotivo:      doc.CodigoMotivo,
		DescripcionMotivo: doc.DescripcionMotivo,
		RefTipoDoc:        doc.Referencia.TipoDoc,
		RefSerieNumero:    doc.Referencia.SerieNumero,
	}, tipoDoc)
}

// Compara montos como decimales para que "118" y "118.00" cuenten como iguales.
func mismoImporte(a, b string) bool {
	return normalizar(a) == normalizar(b)
}

func normalizar(s string) string {
	entero, decimal, tieneComa := strings.Cut(s, ".")
	if !tieneComa {
		return s
	}
	if decimal = strings.TrimRight(decimal, "0"); decimal == "" {
		return entero
	}
	return entero + "." + decimal
}
