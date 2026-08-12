package command

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"facturacion-service/internal/features/comprobante/domain"
	domainerr "facturacion-service/internal/shared/domain/errors"
	"facturacion-service/pkg/ulid"
)

// Los cuatro casos de uso del resumen diario estaban verificados a mano contra
// SUNAT pero sin ningun test. Para camino de dinero es demasiado delgado.

func boletaPendiente(serie string, correlativo int64, total string) *domain.Comprobante {
	return domain.Reconstruir(domain.EstadoPersistido{
		ID:           string(ulid.New()),
		TenantID:     "tenant-1",
		TipoDoc:      domain.TipoBoleta,
		Serie:        serie,
		Correlativo:  correlativo,
		Estado:       domain.EstadoPendienteResumen,
		FechaEmision: domain.HoyEnLima(),
		ImporteTotal: total,
		Payload: json.RawMessage(`{
			"receptor": {"tipo_doc": "0", "num_doc": "-"},
			"totales": {"oper_gravadas": 100.00, "igv": 18.00, "importe_total": ` + total + `}
		}`),
	})
}

// ---------- ArmarResumen ----------

func TestArmarResumenAgrupaLasBoletasYEncolaElEnvio(t *testing.T) {
	res, cola := nuevoResumenFake(), &colaResumenFake{}
	res.boletas = []*domain.Comprobante{
		boletaPendiente("B001", 1, "118.00"),
		boletaPendiente("B001", 2, "236.00"),
	}

	uc := NewArmarResumen(res, cola, txFake{})

	resumen, err := uc.Execute(context.Background(), "tenant-1", domain.HoyEnLima())
	if err != nil {
		t.Fatalf("armar resumen: %v", err)
	}

	if resumen.Tipo() != domain.ResumenDiario {
		t.Fatalf("tipo = %q, esperaba RC", resumen.Tipo())
	}
	if n := len(res.asignados[resumen.ID()]); n != 2 {
		t.Fatalf("asigno %d boletas al resumen, esperaba 2", n)
	}
	if len(cola.enviados) != 1 {
		t.Fatalf("encolo %d envios, esperaba 1", len(cola.enviados))
	}
}

// Sin boletas no hay nada que informar: crear un resumen vacio seria un rechazo
// seguro de SUNAT y quemaria un correlativo de resumen.
func TestArmarResumenSinBoletasNoCreaNada(t *testing.T) {
	res, cola := nuevoResumenFake(), &colaResumenFake{}
	uc := NewArmarResumen(res, cola, txFake{})

	_, err := uc.Execute(context.Background(), "tenant-1", domain.HoyEnLima())
	if err == nil {
		t.Fatal("creo un resumen sin boletas")
	}
	if domErr, ok := domainerr.AsError(err); !ok || domErr.Code() != "SIN_BOLETAS" {
		t.Fatalf("esperaba SIN_BOLETAS, obtuve %v", err)
	}
	if len(res.resumenes) != 0 || len(cola.enviados) != 0 {
		t.Fatal("dejo un resumen o un job huerfano")
	}
}

// El identificador es el que SUNAT espera: RC-AAAAMMDD-correlativo.
func TestIdentificadorDelResumen(t *testing.T) {
	res := nuevoResumenFake()
	res.boletas = []*domain.Comprobante{boletaPendiente("B001", 1, "118.00")}

	fecha := time.Date(2026, 8, 5, 12, 0, 0, 0, domain.Lima)
	resumen, err := NewArmarResumen(res, &colaResumenFake{}, txFake{}).
		Execute(context.Background(), "tenant-1", fecha)
	if err != nil {
		t.Fatal(err)
	}

	if got := resumen.Identificador(); got != "RC-20260805-1" {
		t.Fatalf("identificador = %q, esperaba RC-20260805-1", got)
	}
}

// ---------- EnviarResumen ----------

func TestEnviarResumenGuardaElTicket(t *testing.T) {
	res := nuevoResumenFake()
	res.boletas = []*domain.Comprobante{boletaPendiente("B001", 1, "118.00")}

	resumen, err := NewArmarResumen(res, &colaResumenFake{}, txFake{}).
		Execute(context.Background(), "tenant-1", domain.HoyEnLima())
	if err != nil {
		t.Fatal(err)
	}

	if err := NewEnviarResumen(res, tenantsFake{}, &motorFake{}).
		Execute(context.Background(), resumen.ID()); err != nil {
		t.Fatalf("enviar resumen: %v", err)
	}

	if resumen.Ticket() == "" {
		t.Fatal("no guardo el ticket que devolvio SUNAT")
	}
	if resumen.Estado() != domain.EstadoTicketPendiente {
		t.Fatalf("estado = %q, esperaba ticket_pendiente", resumen.Estado())
	}
}

// El mismo job puede llegar dos veces. El segundo no debe reenviar a SUNAT: se
// duplicaria el resumen y con el todas sus boletas.
func TestEnviarResumenDosVecesNoDuplicaElEnvio(t *testing.T) {
	res := nuevoResumenFake()
	res.boletas = []*domain.Comprobante{boletaPendiente("B001", 1, "118.00")}
	motor := &motorFake{}

	resumen, err := NewArmarResumen(res, &colaResumenFake{}, txFake{}).
		Execute(context.Background(), "tenant-1", domain.HoyEnLima())
	if err != nil {
		t.Fatal(err)
	}

	uc := NewEnviarResumen(res, tenantsFake{}, motor)

	if err := uc.Execute(context.Background(), resumen.ID()); err != nil {
		t.Fatalf("primer envio: %v", err)
	}

	// El segundo worker no puede tomarlo, y como ya tiene ticket cierra el job.
	if err := uc.Execute(context.Background(), resumen.ID()); err != nil {
		t.Fatalf("el segundo envio deberia cerrarse sin error: %v", err)
	}

	if motor.resumenes != 1 {
		t.Fatalf("se envio %d veces el mismo resumen a SUNAT", motor.resumenes)
	}
}

// ---------- ConsultarTicket ----------

func TestConsultarTicketNoListoNoTocaElResumen(t *testing.T) {
	res, resumen := resumenConTicket(t)
	motor := &motorFake{ticketRespuesta: &domain.ResultadoEmision{Estado: "pendiente"}}

	err := NewConsultarTicket(res, tenantsFake{}, motor).
		Execute(context.Background(), resumen.ID())

	if domErr, ok := domainerr.AsError(err); !ok || domErr.Code() != "TICKET_NO_LISTO" {
		t.Fatalf("esperaba TICKET_NO_LISTO, obtuve %v", err)
	}
	if resumen.Estado() != domain.EstadoTicketPendiente {
		t.Fatalf("cambio el estado a %q sin respuesta de SUNAT", resumen.Estado())
	}
	if len(res.resueltos) != 0 {
		t.Fatal("resolvio boletas sin que SUNAT se pronunciara")
	}
}

// El CDR del resumen resuelve de una vez todas las boletas que iban dentro.
func TestConsultarTicketResuelveTodasLasBoletas(t *testing.T) {
	res, resumen := resumenConTicket(t)
	motor := &motorFake{ticketRespuesta: &domain.ResultadoEmision{
		Estado: "aceptado", Codigo: "0", Mensaje: "aceptado", CDR: []byte("cdr"),
	}}

	if err := NewConsultarTicket(res, tenantsFake{}, motor).
		Execute(context.Background(), resumen.ID()); err != nil {
		t.Fatalf("consultar ticket: %v", err)
	}

	if resumen.Estado() != domain.EstadoAceptado {
		t.Fatalf("resumen quedo en %q", resumen.Estado())
	}
	if len(res.resueltos) != len(res.boletas) {
		t.Fatalf("resolvio %d boletas de %d", len(res.resueltos), len(res.boletas))
	}
	for id, estado := range res.resueltos {
		if estado != domain.EstadoAceptado {
			t.Fatalf("boleta %s quedo en %q", id, estado)
		}
	}
}

// Un fallo de transporte no es un dictamen de SUNAT: propagarlo dejaria a las
// boletas en un estado que nadie dicto.
func TestConsultarTicketNoPropagaFalloDeTransporte(t *testing.T) {
	res, resumen := resumenConTicket(t)
	motor := &motorFake{ticketRespuesta: &domain.ResultadoEmision{
		Estado: "rechazado", Codigo: "HTTP", Mensaje: "Unauthorized",
	}}

	err := NewConsultarTicket(res, tenantsFake{}, motor).
		Execute(context.Background(), resumen.ID())
	if err == nil {
		t.Fatal("dio por bueno un fallo de red: no habria reintento")
	}

	if len(res.resueltos) != 0 {
		t.Fatal("marco las boletas con un estado que SUNAT nunca dicto")
	}
	if resumen.Estado().EsFinal() {
		t.Fatalf("el resumen quedo en %q, que es final: no se reintentaria", resumen.Estado())
	}
}

func resumenConTicket(t *testing.T) (*resumenFake, *domain.Resumen) {
	t.Helper()

	res := nuevoResumenFake()
	res.boletas = []*domain.Comprobante{
		boletaPendiente("B001", 1, "118.00"),
		boletaPendiente("B001", 2, "236.00"),
	}

	resumen, err := NewArmarResumen(res, &colaResumenFake{}, txFake{}).
		Execute(context.Background(), "tenant-1", domain.HoyEnLima())
	if err != nil {
		t.Fatal(err)
	}

	if err := NewEnviarResumen(res, tenantsFake{}, &motorFake{}).
		Execute(context.Background(), resumen.ID()); err != nil {
		t.Fatal(err)
	}

	return res, resumen
}

// ---------- Notificar ----------

type webhookFake struct {
	llamadas int
	url      string
	cuerpo   []byte
	firma    string
	fallar   error
}

func (w *webhookFake) Enviar(_ context.Context, url string, cuerpo []byte, firma string) error {
	w.llamadas++
	w.url, w.cuerpo, w.firma = url, cuerpo, firma
	return w.fallar
}

func comprobanteParaAvisar(repo *repoFake, estado domain.Estado) *domain.Comprobante {
	c := comprobanteEn(repo, domain.TipoFactura, "F001", estado)
	c.Resolver("0", "La Factura F001-1 ha sido aceptada", nil, []byte("cdr"))
	repo.comprobantes[c.ID()] = c
	return c
}

// Avisar de estados intermedios genera ruido y obliga al cliente a filtrarlos.
func TestNotificarSoloEnEstadoFinal(t *testing.T) {
	casos := []struct {
		estado domain.Estado
		avisa  bool
	}{
		{domain.EstadoAceptado, true},
		{domain.EstadoRechazado, true},
		{domain.EstadoAnulado, true},
		{domain.EstadoPendiente, false},
		{domain.EstadoProcesando, false},
		{domain.EstadoTicketPendiente, false},
		{domain.EstadoError, false},
	}

	for _, caso := range casos {
		t.Run(string(caso.estado), func(t *testing.T) {
			repo, envio := nuevoRepoFake(), &webhookFake{}
			c := comprobanteEn(repo, domain.TipoFactura, "F001", caso.estado)

			if err := NewNotificar(repo, tenantsConWebhook{}, envio).
				Execute(context.Background(), "tenant-1", c.ID()); err != nil {
				t.Fatalf("notificar: %v", err)
			}

			if aviso := envio.llamadas > 0; aviso != caso.avisa {
				t.Fatalf("estado %q: aviso=%v, esperaba %v", caso.estado, aviso, caso.avisa)
			}
		})
	}
}

// La firma es lo unico que le permite al cliente saber que el webhook viene de
// nosotros y no fue alterado en el camino.
func TestNotificarFirmaVerificable(t *testing.T) {
	repo, envio := nuevoRepoFake(), &webhookFake{}
	c := comprobanteParaAvisar(repo, domain.EstadoAceptado)

	if err := NewNotificar(repo, tenantsConWebhook{}, envio).
		Execute(context.Background(), "tenant-1", c.ID()); err != nil {
		t.Fatalf("notificar: %v", err)
	}

	if envio.llamadas != 1 {
		t.Fatalf("envio %d webhooks", envio.llamadas)
	}

	mac := hmac.New(sha256.New, []byte(secretoDePrueba))
	mac.Write(envio.cuerpo)
	esperada := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if envio.firma != esperada {
		t.Fatalf("firma = %q, esperaba %q", envio.firma, esperada)
	}
	if !strings.HasPrefix(envio.firma, "sha256=") {
		t.Fatalf("la firma debe declarar el algoritmo: %q", envio.firma)
	}

	var evento EventoWebhook
	if err := json.Unmarshal(envio.cuerpo, &evento); err != nil {
		t.Fatalf("el cuerpo no es JSON valido: %v", err)
	}
	if evento.Evento != "comprobante.aceptado" {
		t.Fatalf("evento = %q", evento.Evento)
	}
	if evento.Numero != c.Numero() {
		t.Fatalf("numero = %q, esperaba %q", evento.Numero, c.Numero())
	}
}

// Un cuerpo alterado no puede seguir validando contra la misma firma.
func TestFirmaCambiaConElCuerpo(t *testing.T) {
	a := Firmar([]byte(`{"estado":"aceptado"}`), secretoDePrueba)
	b := Firmar([]byte(`{"estado":"rechazado"}`), secretoDePrueba)

	if a == b {
		t.Fatal("la misma firma vale para dos cuerpos distintos")
	}
	if Firmar([]byte(`{"estado":"aceptado"}`), "otro-secreto") == a {
		t.Fatal("la firma no depende del secreto")
	}
}

// Un emisor sin webhook configurado no debe generar intentos de envio.
func TestNotificarSinWebhookNoHaceNada(t *testing.T) {
	repo, envio := nuevoRepoFake(), &webhookFake{}
	c := comprobanteParaAvisar(repo, domain.EstadoAceptado)

	if err := NewNotificar(repo, tenantsFake{}, envio).
		Execute(context.Background(), "tenant-1", c.ID()); err != nil {
		t.Fatalf("notificar: %v", err)
	}

	if envio.llamadas != 0 {
		t.Fatal("intento enviar un webhook sin URL configurada")
	}
}

// Si el webhook falla, el comprobante no puede quedar marcado como notificado:
// el barredor debe poder reintentarlo.
func TestWebhookFallidoNoSeMarcaComoEnviado(t *testing.T) {
	repo := nuevoRepoFake()
	envio := &webhookFake{fallar: domainerr.Internal("timeout")}
	c := comprobanteParaAvisar(repo, domain.EstadoAceptado)

	if err := NewNotificar(repo, tenantsConWebhook{}, envio).
		Execute(context.Background(), "tenant-1", c.ID()); err == nil {
		t.Fatal("dio por enviado un webhook que fallo")
	}

	if repo.notificados[c.ID()] {
		t.Fatal("marco como notificado un webhook que nunca llego")
	}
}

const secretoDePrueba = "secreto-de-prueba"

type tenantsConWebhook struct{}

func (tenantsConWebhook) PorID(context.Context, string) (*domain.Tenant, error) {
	return &domain.Tenant{
		ID: "tenant-1", RUC: "20000000001",
		WebhookURL: "https://cliente.example/webhook", WebhookSecret: secretoDePrueba,
	}, nil
}
func (tenantsConWebhook) PorAPIKey(context.Context, string) (*domain.Tenant, error) {
	return nil, nil
}
