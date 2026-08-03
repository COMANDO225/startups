package command

import (
	"context"
	"testing"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
	"facturacion-service/pkg/ulid"
)

type resumenFake struct {
	correlativos map[string]int64
	resumenes    map[string]*domain.Resumen
	asignados    map[string][]string
}

func nuevoResumenFake() *resumenFake {
	return &resumenFake{
		correlativos: map[string]int64{},
		resumenes:    map[string]*domain.Resumen{},
		asignados:    map[string][]string{},
	}
}

func (r *resumenFake) SiguienteCorrelativoResumen(_ context.Context, tenantID string, tipo domain.TipoResumen, f time.Time) (int64, error) {
	k := tenantID + "|" + string(tipo) + "|" + f.Format("2006-01-02")
	r.correlativos[k]++
	return r.correlativos[k], nil
}

func (r *resumenFake) CrearResumen(_ context.Context, res *domain.Resumen) error {
	r.resumenes[res.ID()] = res
	return nil
}

func (r *resumenFake) ResumenPorID(_ context.Context, id string) (*domain.Resumen, error) {
	res, ok := r.resumenes[id]
	if !ok {
		return nil, domain.ErrResumenNoEncontrado()
	}
	return res, nil
}

func (r *resumenFake) TomarResumen(_ context.Context, id string) (*domain.Resumen, error) {
	return r.ResumenPorID(context.Background(), id)
}
func (r *resumenFake) ActualizarResumen(context.Context, *domain.Resumen) error { return nil }
func (r *resumenFake) ResumenesEnCurso(context.Context, time.Duration, int) ([]*domain.Resumen, error) {
	return nil, nil
}
func (r *resumenFake) GruposPendientes(context.Context, int) ([]domain.GrupoBoletas, error) {
	return nil, nil
}
func (r *resumenFake) BoletasParaResumen(context.Context, string, time.Time, int) ([]*domain.Comprobante, error) {
	return nil, nil
}

func (r *resumenFake) AsignarResumen(_ context.Context, resumenID string, ids []string) error {
	r.asignados[resumenID] = ids
	return nil
}
func (r *resumenFake) ResolverComprobantesDeResumen(context.Context, string, domain.Estado, string, string) error {
	return nil
}
func (r *resumenFake) ComprobantesDeResumen(context.Context, string) ([]*domain.Comprobante, error) {
	return nil, nil
}

type colaResumenFake struct{ enviados []string }

func (c *colaResumenFake) EncolarEnvioResumen(_ context.Context, id string) error {
	c.enviados = append(c.enviados, id)
	return nil
}
func (c *colaResumenFake) EncolarConsultaTicket(context.Context, string) error { return nil }

func comprobanteEn(repo *repoFake, tipo domain.TipoDoc, serie string, estado domain.Estado) *domain.Comprobante {
	c := domain.Reconstruir(domain.EstadoPersistido{
		ID:           string(ulid.New()),
		TenantID:     "tenant-1",
		TipoDoc:      tipo,
		Serie:        serie,
		Correlativo:  1,
		Estado:       estado,
		FechaEmision: time.Now(),
		Payload:      []byte(`{"totales":{"importe_total":118}}`),
	})
	repo.comprobantes[c.ID()] = c
	return c
}

// Una boleta que nunca llego a SUNAT no necesita comunicarse: se marca y listo.
func TestAnularComprobanteNoInformadoNoGeneraResumen(t *testing.T) {
	repo, res, cola := nuevoRepoFake(), nuevoResumenFake(), &colaResumenFake{}
	uc := NewAnular(repo, res, cola, txFake{})

	c := comprobanteEn(repo, domain.TipoBoleta, "B001", domain.EstadoPendienteResumen)

	if _, err := uc.Execute(context.Background(), AnularCmd{
		TenantID: "tenant-1", ComprobanteID: c.ID(), Motivo: "error de digitacion",
	}); err != nil {
		t.Fatalf("anular: %v", err)
	}

	if len(res.resumenes) != 0 {
		t.Fatal("creo un resumen para algo que SUNAT nunca recibio")
	}
	if len(cola.enviados) != 0 {
		t.Fatal("encolo un envio innecesario a SUNAT")
	}
	if !repo.anulados[c.ID()] {
		t.Fatal("no marco el comprobante como anulado")
	}
}

// La respuesta al cliente debe mostrar el estado nuevo, no el anterior.
func TestAnularDevuelveEntidadActualizada(t *testing.T) {
	repo := nuevoRepoFake()
	uc := NewAnular(repo, nuevoResumenFake(), &colaResumenFake{}, txFake{})

	c := comprobanteEn(repo, domain.TipoFactura, "F001", domain.EstadoAceptado)

	devuelto, err := uc.Execute(context.Background(), AnularCmd{
		TenantID: "tenant-1", ComprobanteID: c.ID(), Motivo: "error en el monto",
	})
	if err != nil {
		t.Fatalf("anular: %v", err)
	}

	if devuelto.Estado() != domain.EstadoAnulado {
		t.Fatalf("devolvio estado %q, esperaba anulado", devuelto.Estado())
	}
	if devuelto.MotivoBaja() != "error en el monto" {
		t.Fatalf("no propago el motivo: %q", devuelto.MotivoBaja())
	}
}

// Una factura aceptada exige Comunicacion de Baja (RA).
func TestAnularFacturaAceptadaGeneraRA(t *testing.T) {
	repo, res, cola := nuevoRepoFake(), nuevoResumenFake(), &colaResumenFake{}
	uc := NewAnular(repo, res, cola, txFake{})

	c := comprobanteEn(repo, domain.TipoFactura, "F001", domain.EstadoAceptado)

	if _, err := uc.Execute(context.Background(), AnularCmd{
		TenantID: "tenant-1", ComprobanteID: c.ID(), Motivo: "anulacion de la operacion",
	}); err != nil {
		t.Fatalf("anular: %v", err)
	}

	if len(res.resumenes) != 1 {
		t.Fatalf("esperaba 1 resumen, hay %d", len(res.resumenes))
	}
	for _, r := range res.resumenes {
		if r.Tipo() != domain.ResumenBaja {
			t.Fatalf("una factura debe anularse con RA, se uso %q", r.Tipo())
		}
	}
	if len(cola.enviados) != 1 {
		t.Fatal("no encolo el envio de la baja")
	}
}

// Una boleta ya aceptada se anula informandola en un RC con estado 3.
func TestAnularBoletaAceptadaGeneraRC(t *testing.T) {
	repo, res, cola := nuevoRepoFake(), nuevoResumenFake(), &colaResumenFake{}
	uc := NewAnular(repo, res, cola, txFake{})

	c := comprobanteEn(repo, domain.TipoBoleta, "B001", domain.EstadoAceptado)

	if _, err := uc.Execute(context.Background(), AnularCmd{
		TenantID: "tenant-1", ComprobanteID: c.ID(), Motivo: "devolucion del cliente",
	}); err != nil {
		t.Fatalf("anular: %v", err)
	}

	for _, r := range res.resumenes {
		if r.Tipo() != domain.ResumenDiario {
			t.Fatalf("una boleta debe anularse con RC, se uso %q", r.Tipo())
		}
	}
}

func TestAnularRechazaEstadosImposibles(t *testing.T) {
	casos := []struct {
		estado domain.Estado
		codigo string
	}{
		{domain.EstadoAnulado, "YA_ANULADO"},
		{domain.EstadoRechazado, "NO_ANULABLE"},
		{domain.EstadoProcesando, "ANULAR_EN_VUELO"},
		{domain.EstadoTicketPendiente, "ANULAR_EN_VUELO"},
	}

	for _, caso := range casos {
		t.Run(string(caso.estado), func(t *testing.T) {
			repo := nuevoRepoFake()
			uc := NewAnular(repo, nuevoResumenFake(), &colaResumenFake{}, txFake{})

			c := comprobanteEn(repo, domain.TipoFactura, "F001", caso.estado)

			_, err := uc.Execute(context.Background(), AnularCmd{
				TenantID: "tenant-1", ComprobanteID: c.ID(), Motivo: "x",
			})
			if err == nil {
				t.Fatalf("acepto anular un comprobante en %q", caso.estado)
			}
			if domErr, ok := domainerr.AsError(err); !ok || domErr.Code() != caso.codigo {
				t.Fatalf("esperaba %s, obtuve %v", caso.codigo, err)
			}
		})
	}
}
