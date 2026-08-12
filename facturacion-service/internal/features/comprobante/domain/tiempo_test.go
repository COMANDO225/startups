package domain

import (
	"testing"
	"time"
)

// El bug: Greenter renderiza el XML en America/Lima siempre. Al enviarle la
// medianoche UTC del dia 6, escribia "2026-08-05 19:00" y el documento legal
// quedaba con una fecha distinta a la registrada en la base.
func TestFechaEmisionNoSeCorreDeDia(t *testing.T) {
	// Tal como sale de una columna DATE de Postgres.
	fecha := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	enLima := FechaEmisionLima(fecha)

	if got := enLima.Format("2006-01-02"); got != "2026-08-06" {
		t.Fatalf("la fecha se corrio a %s", got)
	}

	// Lo que ve Greenter despues de convertir a America/Lima.
	if got := enLima.In(Lima).Format("2006-01-02"); got != "2026-08-06" {
		t.Fatalf("al renderizar en Lima la fecha se corrio a %s", got)
	}

	// Y tambien tiene que aguantar la ida y vuelta por UTC.
	if got := enLima.UTC().In(Lima).Format("2006-01-02"); got != "2026-08-06" {
		t.Fatalf("tras pasar por UTC la fecha se corrio a %s", got)
	}
}

// Una venta de las 22:00 en Lima es de ese dia, aunque en UTC ya sea el
// siguiente. Registrarla en UTC le cambiaria el dia al comprobante.
func TestHoyEnLimaUsaElDiaPeruano(t *testing.T) {
	nocheEnLima := time.Date(2026, 8, 5, 22, 30, 0, 0, Lima)

	if got := nocheEnLima.UTC().Format("2006-01-02"); got != "2026-08-06" {
		t.Fatalf("premisa rota: en UTC deberia ser el dia 6, es %s", got)
	}

	if got := FechaEmisionLima(nocheEnLima).Format("2006-01-02"); got != "2026-08-05" {
		t.Fatalf("la venta de las 22:00 quedo registrada el %s", got)
	}
}

// Sin tzdata embebido, LoadLocation falla en la imagen del contenedor y todo
// cae a UTC sin avisar.
func TestZonaDeLimaDisponible(t *testing.T) {
	_, offset := time.Date(2026, 8, 5, 12, 0, 0, 0, Lima).Zone()
	if offset != -5*60*60 {
		t.Fatalf("desfase de Lima = %ds, esperaba -18000 (UTC-5)", offset)
	}
}
