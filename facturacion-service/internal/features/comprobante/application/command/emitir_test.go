package command

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
)

// repoFake simula la unicidad que en produccion garantizan los constraints
// uq_comprobantes_idempotency y uq_comprobantes_numeracion.
type repoFake struct {
	correlativo  int64
	comprobantes map[string]*domain.Comprobante
	porKey       map[string]*domain.Comprobante
	tomados      map[string]bool
	anulados     map[string]bool
	notificados  map[string]bool
}

func nuevoRepoFake() *repoFake {
	return &repoFake{
		comprobantes: map[string]*domain.Comprobante{},
		porKey:       map[string]*domain.Comprobante{},
		tomados:      map[string]bool{},
		anulados:     map[string]bool{},
		notificados:  map[string]bool{},
	}
}

func (r *repoFake) SiguienteCorrelativo(_ context.Context, _ string, _ domain.TipoDoc, _ string) (int64, error) {
	r.correlativo++
	return r.correlativo, nil
}

func (r *repoFake) Crear(_ context.Context, c *domain.Comprobante) error {
	r.comprobantes[c.ID()] = c
	r.porKey[c.TenantID()+"|"+c.IdempotencyKey()] = c
	return nil
}

func (r *repoFake) PorID(_ context.Context, _, id string) (*domain.Comprobante, error) {
	c, ok := r.comprobantes[id]
	if !ok {
		return nil, domain.ErrNoEncontrado()
	}
	return c, nil
}

func (r *repoFake) PorIdempotencyKey(_ context.Context, tenantID, key string) (*domain.Comprobante, error) {
	c, ok := r.porKey[tenantID+"|"+key]
	if !ok {
		return nil, domain.ErrNoEncontrado()
	}
	return c, nil
}

func (r *repoFake) Listar(context.Context, string, int, int) ([]*domain.Comprobante, int, error) {
	return nil, 0, nil
}

func (r *repoFake) Tomar(_ context.Context, id string) (*domain.Comprobante, error) {
	if r.tomados[id] {
		return nil, domain.ErrYaTomado()
	}
	r.tomados[id] = true
	return r.comprobantes[id], nil
}

func (r *repoFake) Actualizar(_ context.Context, c *domain.Comprobante) error {
	r.comprobantes[c.ID()] = c
	return nil
}

type colaFake struct{ encolados []string }

func (c *colaFake) EncolarEmision(_ context.Context, id string) error {
	c.encolados = append(c.encolados, id)
	return nil
}

type txFake struct{}

func (txFake) RunInTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// payloadValido: lo minimo que SUNAT acepta. Los tests que prueban rechazos
// parten de aqui y rompen un campo a la vez.
func payloadValido(importe string) json.RawMessage {
	return json.RawMessage(`{
		"totales": {"importe_total": ` + importe + `},
		"receptor": {"tipo_doc": "6", "num_doc": "20000000001", "razon_social": "CLIENTE DE PRUEBA SAC"},
		"items": [{"descripcion": "menu del dia"}]
	}`)
}

func cmdBase() EmitirCmd {
	return EmitirCmd{
		TenantID:       "tenant-1",
		IdempotencyKey: "venta-001",
		TipoDoc:        "01",
		Serie:          "F001",
		Payload:        payloadValido("118.00"),
		Moneda:         "PEN",
		ImporteTotal:   "118.00",
		FechaEmision:   time.Now(),
	}
}

// boletaBase: la venta tipica de un restaurante, a consumidor final sin
// documento. No puede rechazarse.
func boletaBase() EmitirCmd {
	cmd := cmdBase()
	cmd.TipoDoc = "03"
	cmd.Serie = "B001"
	cmd.Payload = json.RawMessage(`{
		"totales": {"importe_total": 25.00},
		"receptor": {"tipo_doc": "0", "num_doc": "-", "razon_social": "VARIOS"},
		"items": [{"descripcion": "menu del dia"}]
	}`)
	cmd.ImporteTotal = "25.00"
	return cmd
}

// El caso que motiva todo el diseño: un retry del cliente no puede generar un
// segundo comprobante ni quemar otro correlativo.
func TestEmitirDosVecesMismaClaveNoDuplica(t *testing.T) {
	repo, cola := nuevoRepoFake(), &colaFake{}
	uc := NewEmitir(repo, cola, txFake{})

	primero, err := uc.Execute(context.Background(), cmdBase())
	if err != nil {
		t.Fatalf("primera emision: %v", err)
	}

	segundo, err := uc.Execute(context.Background(), cmdBase())
	if err != nil {
		t.Fatalf("segunda emision: %v", err)
	}

	if primero.ID() != segundo.ID() {
		t.Fatalf("se crearon dos comprobantes: %s y %s", primero.ID(), segundo.ID())
	}
	if primero.Correlativo() != segundo.Correlativo() {
		t.Fatalf("se quemaron dos correlativos: %d y %d", primero.Correlativo(), segundo.Correlativo())
	}
	if len(repo.comprobantes) != 1 {
		t.Fatalf("esperaba 1 comprobante, hay %d", len(repo.comprobantes))
	}
	if len(cola.encolados) != 1 {
		t.Fatalf("esperaba 1 job encolado, hay %d", len(cola.encolados))
	}
}

func TestEmitirClavesDistintasAvanzaCorrelativo(t *testing.T) {
	repo := nuevoRepoFake()
	uc := NewEmitir(repo, &colaFake{}, txFake{})

	c1 := cmdBase()
	c2 := cmdBase()
	c2.IdempotencyKey = "venta-002"

	primero, _ := uc.Execute(context.Background(), c1)
	segundo, _ := uc.Execute(context.Background(), c2)

	if segundo.Correlativo() != primero.Correlativo()+1 {
		t.Fatalf("correlativo no avanzo: %d -> %d", primero.Correlativo(), segundo.Correlativo())
	}
}

// Todo lo rechazable debe rechazarse ANTES de tocar el contador: un correlativo
// consumido por un documento invalido deja un hueco que SUNAT observa.
func TestEmitirValidaAntesDeQuemarCorrelativo(t *testing.T) {
	casos := []struct {
		nombre string
		mutar  func(*EmitirCmd)
	}{
		{"tipo_doc invalido", func(c *EmitirCmd) { c.TipoDoc = "99" }},
		{"serie de boleta en factura", func(c *EmitirCmd) { c.Serie = "B001" }},
		{"serie con largo invalido", func(c *EmitirCmd) { c.Serie = "F1" }},
		{"fecha en el futuro", func(c *EmitirCmd) { c.FechaEmision = time.Now().Add(72 * time.Hour) }},
		{"payload sin totales", func(c *EmitirCmd) { c.Payload = json.RawMessage(`{}`) }},
		{"payload ilegible", func(c *EmitirCmd) { c.Payload = json.RawMessage(`no es json`) }},
		{"importe distinto al del payload", func(c *EmitirCmd) { c.ImporteTotal = "999.00" }},

		// Los que SUNAT nos rechazo en la prueba de carga: 10 de 30 se fueron
		// con codigo 2022 y el correlativo se quemo igual.
		{"razon social de un caracter", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"6","num_doc":"20000000001","razon_social":"C"`)
		}},
		{"razon social vacia", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"6","num_doc":"20000000001","razon_social":"   "`)
		}},
		{"factura a DNI", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"1","num_doc":"46778912","razon_social":"JUAN PEREZ"`)
		}},
		{"RUC con digito verificador malo", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"6","num_doc":"20000000009","razon_social":"CLIENTE SAC"`)
		}},
		{"RUC de 10 digitos", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"6","num_doc":"2000000000","razon_social":"CLIENTE SAC"`)
		}},
		{"tipo de documento fuera del catalogo", func(c *EmitirCmd) {
			c.Payload = conReceptor(`"tipo_doc":"9","num_doc":"20000000001","razon_social":"CLIENTE SAC"`)
		}},
		{"sin items", func(c *EmitirCmd) {
			c.Payload = json.RawMessage(`{"totales":{"importe_total":118.00},"receptor":{"tipo_doc":"6","num_doc":"20000000001","razon_social":"CLIENTE SAC"},"items":[]}`)
		}},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := nuevoRepoFake()
			uc := NewEmitir(repo, &colaFake{}, txFake{})

			cmd := cmdBase()
			caso.mutar(&cmd)

			if _, err := uc.Execute(context.Background(), cmd); err == nil {
				t.Fatal("acepto un comprobante invalido")
			}
			if repo.correlativo != 0 {
				t.Fatalf("quemo un correlativo en un documento invalido: contador = %d", repo.correlativo)
			}
			if len(repo.comprobantes) != 0 {
				t.Fatal("persistio un comprobante invalido")
			}
		})
	}
}

func conReceptor(receptor string) json.RawMessage {
	return json.RawMessage(`{
		"totales": {"importe_total": 118.00},
		"receptor": {` + receptor + `},
		"items": [{"descripcion": "menu del dia"}]
	}`)
}

// "118" y "118.00" son el mismo monto.
func TestEmitirAceptaImportesEquivalentes(t *testing.T) {
	uc := NewEmitir(nuevoRepoFake(), &colaFake{}, txFake{})

	cmd := cmdBase()
	cmd.ImporteTotal = "118.00"
	cmd.Payload = payloadValido("118")

	if _, err := uc.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("rechazo importes equivalentes: %v", err)
	}
}

// La venta mas comun de un restaurante: boleta chica, cliente sin documento.
// Validar de mas aqui seria peor que no validar.
func TestEmitirAceptaBoletaSinDocumento(t *testing.T) {
	uc := NewEmitir(nuevoRepoFake(), &colaFake{}, txFake{})

	if _, err := uc.Execute(context.Background(), boletaBase()); err != nil {
		t.Fatalf("rechazo una boleta a consumidor final: %v", err)
	}
}

// Pero pasando S/700 SUNAT exige identificar al comprador.
func TestEmitirRechazaBoletaGrandeSinDocumento(t *testing.T) {
	repo := nuevoRepoFake()
	uc := NewEmitir(repo, &colaFake{}, txFake{})

	cmd := boletaBase()
	cmd.Payload = json.RawMessage(`{
		"totales": {"importe_total": 850.00},
		"receptor": {"tipo_doc": "0", "num_doc": "-", "razon_social": "VARIOS"},
		"items": [{"descripcion": "banquete"}]
	}`)
	cmd.ImporteTotal = "850.00"

	if _, err := uc.Execute(context.Background(), cmd); err == nil {
		t.Fatal("acepto una boleta de S/850 sin documento del comprador")
	}
	if repo.correlativo != 0 {
		t.Fatalf("quemo un correlativo: contador = %d", repo.correlativo)
	}
}

// River entrega at-least-once: el mismo job puede llegar dos veces.
func TestTomarDosVecesSoloUnoGana(t *testing.T) {
	repo := nuevoRepoFake()
	uc := NewEmitir(repo, &colaFake{}, txFake{})

	c, _ := uc.Execute(context.Background(), cmdBase())

	if _, err := repo.Tomar(context.Background(), c.ID()); err != nil {
		t.Fatalf("primer worker deberia ganar: %v", err)
	}
	if _, err := repo.Tomar(context.Background(), c.ID()); err == nil {
		t.Fatal("el segundo worker tambien tomo el comprobante")
	}
}

func (r *repoFake) EstadoActual(_ context.Context, id string) (domain.Estado, error) {
	c, ok := r.comprobantes[id]
	if !ok {
		return "", domain.ErrNoEncontrado()
	}
	return c.Estado(), nil
}

func (r *repoFake) Huerfanos(context.Context, time.Duration, int) ([]*domain.Comprobante, error) {
	return nil, nil
}

func (r *repoFake) RequierenAtencion(context.Context, string, int) ([]*domain.Comprobante, error) {
	return nil, nil
}

func (r *repoFake) MarcarWebhookEnviado(_ context.Context, id string) error {
	r.notificados[id] = true
	return nil
}

func (r *repoFake) MarcarAnulado(_ context.Context, id, _ string) error {
	r.anulados[id] = true
	return nil
}

func (r *repoFake) SinNotificar(context.Context, time.Duration, int) ([]*domain.Comprobante, error) {
	return nil, nil
}

type avisosFake struct{ encolados []string }

func (a *avisosFake) EncolarNotificacion(_ context.Context, _, comprobanteID string) error {
	a.encolados = append(a.encolados, comprobanteID)
	return nil
}

type tenantsFake struct{}

func (tenantsFake) PorID(context.Context, string) (*domain.Tenant, error) {
	return &domain.Tenant{ID: "tenant-1", RUC: "20000000001"}, nil
}
func (tenantsFake) PorAPIKey(context.Context, string) (*domain.Tenant, error) { return nil, nil }

type motorFake struct {
	llamadas  int
	resumenes int

	// ticketRespuesta permite simular lo que SUNAT contesta al consultar el
	// ticket: pendiente, aceptado o un fallo de transporte.
	ticketRespuesta *domain.ResultadoEmision
}

func (m *motorFake) Emitir(context.Context, *domain.Tenant, []byte, domain.TipoDoc, string, int64, time.Time) (*domain.ResultadoEmision, error) {
	m.llamadas++
	return &domain.ResultadoEmision{Codigo: "0", Mensaje: "aceptada", XML: []byte("<xml/>"), CDR: []byte("cdr")}, nil
}
func (m *motorFake) ConsultarTicket(context.Context, *domain.Tenant, string) (*domain.ResultadoEmision, error) {
	if m.ticketRespuesta != nil {
		return m.ticketRespuesta, nil
	}
	return &domain.ResultadoEmision{Estado: "pendiente"}, nil
}

func (m *motorFake) Firmar(context.Context, *domain.Tenant, []byte, domain.TipoDoc, string, int64, time.Time) (*domain.ResultadoEmision, error) {
	m.llamadas++
	return &domain.ResultadoEmision{XML: []byte("<xml/>"), Estado: "pendiente_resumen"}, nil
}

func (m *motorFake) EnviarResumen(context.Context, *domain.Tenant, *domain.Resumen, []domain.DetalleResumen) (*domain.ResultadoEmision, error) {
	m.resumenes++
	return &domain.ResultadoEmision{Ticket: "ticket-1", XML: []byte("<rc/>")}, nil
}

func (m *motorFake) EnviarBaja(context.Context, *domain.Tenant, *domain.Resumen, []domain.DetalleBaja) (*domain.ResultadoEmision, error) {
	return &domain.ResultadoEmision{Ticket: "ticket-2"}, nil
}

// El bug que costo 30 minutos de comprobante atascado: cuando otro worker tiene
// el comprobante, cerrar el job lo deja huerfano si ese worker muere. Debe
// pedirse un reintento para que la ventana de rescate llegue a evaluarse.
func TestProcesarPideReintentoSiSigueEnVuelo(t *testing.T) {
	repo := nuevoRepoFake()
	motor := &motorFake{}
	emitir := NewEmitir(repo, &colaFake{}, txFake{})
	procesar := NewProcesar(repo, tenantsFake{}, motor, &avisosFake{})

	c, _ := emitir.Execute(context.Background(), cmdBase())

	if err := procesar.Execute(context.Background(), c.ID()); err != nil {
		t.Fatalf("primer worker: %v", err)
	}
	if motor.llamadas != 1 {
		t.Fatalf("esperaba 1 envio a SUNAT, hubo %d", motor.llamadas)
	}

	// Segundo worker sobre un comprobante ya resuelto: cierra el job.
	if err := procesar.Execute(context.Background(), c.ID()); err != nil {
		t.Fatalf("estado final deberia cerrar el job sin error: %v", err)
	}
	if motor.llamadas != 1 {
		t.Fatalf("se reenvio a SUNAT un comprobante ya resuelto: %d envios", motor.llamadas)
	}
}

func TestProcesarNoCierraJobDeComprobanteEnVuelo(t *testing.T) {
	repo := nuevoRepoFake()
	emitir := NewEmitir(repo, &colaFake{}, txFake{})
	procesar := NewProcesar(repo, tenantsFake{}, &motorFake{}, &avisosFake{})

	c, _ := emitir.Execute(context.Background(), cmdBase())

	// Simula que otro worker lo tomo y sigue trabajando: queda en procesando.
	repo.tomados[c.ID()] = true

	err := procesar.Execute(context.Background(), c.ID())
	if err == nil {
		t.Fatal("cerro el job de un comprobante en vuelo: quedaria huerfano si el otro worker muere")
	}
	if domErr, ok := domainerr.AsError(err); !ok || domErr.Code() != "COMPROBANTE_EN_VUELO" {
		t.Fatalf("esperaba COMPROBANTE_EN_VUELO, obtuve %v", err)
	}
}
