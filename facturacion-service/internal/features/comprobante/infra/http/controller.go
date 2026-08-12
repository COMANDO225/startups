package http

import (
	"time"

	"facturacion-service/internal/features/comprobante/application/command"
	"facturacion-service/internal/features/comprobante/application/query"
	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"

	"github.com/gofiber/fiber/v3"
)

type Controller struct {
	emitir   *command.Emitir
	anular   *command.Anular
	obtener  *query.Obtener
	listar   *query.Listar
	atencion *query.RequierenAtencion
}

func NewController(
	emitir *command.Emitir,
	anular *command.Anular,
	obtener *query.Obtener,
	listar *query.Listar,
	atencion *query.RequierenAtencion,
) *Controller {
	return &Controller{emitir: emitir, anular: anular, obtener: obtener, listar: listar, atencion: atencion}
}

// Anular da de baja el comprobante ante SUNAT. Segun como haya llegado alli, se
// resuelve localmente, con una Comunicacion de Baja o con un Resumen Diario.
func (ctrl *Controller) Anular(c fiber.Ctx) error {
	var req AnularRequest
	if err := c.Bind().Body(&req); err != nil {
		return err
	}

	comprobante, err := ctrl.anular.Execute(c.Context(), command.AnularCmd{
		TenantID:      tenantDe(c).ID,
		ComprobanteID: c.Params("id"),
		Motivo:        req.Motivo,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(toResponse(comprobante, false))
}

// Atencion expone lo que ningun reintento arregla. Es el unico lugar donde un
// comprobante roto se vuelve visible.
func (ctrl *Controller) Atencion(c fiber.Ctx) error {
	comprobantes, err := ctrl.atencion.Execute(c.Context(), tenantDe(c).ID, 100)
	if err != nil {
		return err
	}

	items := make([]ComprobanteResponse, len(comprobantes))
	for i, comp := range comprobantes {
		items[i] = toResponse(comp, false)
	}

	return c.JSON(fiber.Map{"data": items, "meta": fiber.Map{"total": len(items)}})
}

func (ctrl *Controller) Emitir(c fiber.Ctx) error {
	key := c.Get("Idempotency-Key")
	if key == "" {
		return domainerr.Validation("Falta la cabecera Idempotency-Key").
			WithCode("IDEMPOTENCY_KEY_REQUERIDA").
			WithSuggestion("Envie un identificador unico de la venta para que un reintento no genere dos comprobantes")
	}

	// StructValidator valida en el Bind: no hace falta validar aparte.
	var req EmitirRequest
	if err := c.Bind().Body(&req); err != nil {
		return err
	}

	// La fecha de emision es una fecha de calendario peruana: una venta de las
	// 20:00 en Lima es de ese dia, aunque en UTC ya sea el siguiente.
	fecha := domain.HoyEnLima()
	if req.FechaEmision != "" {
		parsed, err := time.ParseInLocation("2006-01-02", req.FechaEmision, domain.Lima)
		if err != nil {
			return domainerr.Validation("Fecha de emision invalida").WithCode("FECHA_INVALIDA")
		}
		fecha = parsed
	}

	moneda := req.Moneda
	if moneda == "" {
		moneda = "PEN"
	}

	comprobante, err := ctrl.emitir.Execute(c.Context(), command.EmitirCmd{
		TenantID:       tenantDe(c).ID,
		IdempotencyKey: key,
		TipoDoc:        req.TipoDoc,
		Serie:          req.Serie,
		Payload:        req.Comprobante,
		Moneda:         moneda,
		ImporteTotal:   req.ImporteTotal,
		FechaEmision:   fecha,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(toResponse(comprobante, false))
}

func (ctrl *Controller) Obtener(c fiber.Ctx) error {
	comprobante, err := ctrl.obtener.Execute(c.Context(), tenantDe(c).ID, c.Params("id"))
	if err != nil {
		return err
	}

	resp := toResponse(comprobante, true)
	resp.QR, resp.Hash = datosQR(comprobante, tenantDe(c))

	return c.JSON(resp)
}

func (ctrl *Controller) Listar(c fiber.Ctx) error {
	limit := fiber.Query(c, "limit", 20)
	page := fiber.Query(c, "page", 1)
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}

	comprobantes, total, err := ctrl.listar.Execute(c.Context(), tenantDe(c).ID, limit, (page-1)*limit)
	if err != nil {
		return err
	}

	items := make([]ComprobanteResponse, len(comprobantes))
	for i, comp := range comprobantes {
		items[i] = toResponse(comp, false)
	}

	return c.JSON(fiber.Map{
		"data": items,
		"meta": fiber.Map{"page": page, "limit": limit, "total": total},
	})
}

func tenantDe(c fiber.Ctx) *domain.Tenant {
	return fiber.Locals[*domain.Tenant](c, ctxKeyTenant)
}
