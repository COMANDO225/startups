package command

import (
	"context"
	"testing"

	"facturacion-service/internal/features/comprobante/domain"
)

// El bug: si el motor devolvia ticket para un comprobante individual, este
// quedaba en 'ticket_pendiente' — un estado SIN SALIDA. Ningun worker consulta
// el ticket de un comprobante (solo el de resumenes) y el barrido de huerfanos
// no cubre ese estado. El comprobante se quedaba ahi para siempre.
func TestTicketInesperadoNoDejaElComprobanteSinSalida(t *testing.T) {
	repo := nuevoRepoFake()
	motor := &motorFake{ticketEnEmitir: "ticket-que-nadie-consulta"}
	emitir := NewEmitir(repo, &colaFake{}, txFake{})
	procesar := NewProcesar(repo, tenantsFake{}, motor, &avisosFake{})

	c, err := emitir.Execute(context.Background(), cmdBase())
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}

	_ = procesar.Execute(context.Background(), c.ID())

	estado := repo.comprobantes[c.ID()].Estado()

	if estado == domain.EstadoTicketPendiente {
		t.Fatal("quedo en ticket_pendiente: ningun worker lo saca de ahi")
	}

	// Debe quedar en un estado del que alguien lo pueda sacar: o retomable, o
	// final y visible.
	tomables := map[string]bool{}
	for _, e := range domain.EstadosTomables() {
		tomables[e] = true
	}
	if !tomables[string(estado)] && !estado.EsFinal() {
		t.Fatalf("estado %q no es tomable ni final: nadie lo saca de ahi", estado)
	}
}
