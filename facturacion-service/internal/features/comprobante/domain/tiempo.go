package domain

import (
	"time"

	// La imagen del contenedor no trae la base de zonas horarias; embeberla
	// evita que LoadLocation falle y caiga a UTC sin avisar.
	_ "time/tzdata"
)

// Lima es la zona de referencia de todo el servicio. La fecha de emision de un
// comprobante es una fecha de calendario peruana: una venta de las 20:00 en Lima
// pertenece a ese dia, aunque en UTC ya sea el siguiente.
var Lima = cargarLima()

func cargarLima() *time.Location {
	loc, err := time.LoadLocation("America/Lima")
	if err != nil {
		// -5 fijo: Peru no aplica horario de verano.
		return time.FixedZone("-05", -5*60*60)
	}
	return loc
}

// HoyEnLima es la fecha de emision por defecto cuando el cliente no la declara.
func HoyEnLima() time.Time {
	return time.Now().In(Lima)
}

// FechaEmisionLima fija una fecha de calendario al mediodia de Lima.
//
// Recibe el dia tal cual, SIN convertir de zona: lo que llega de la base es una
// fecha de calendario ("el 6 de agosto"), no un instante, y convertirla seria
// justo el error que esto evita.
//
// Hace falta porque Greenter renderiza el XML con TimeZonePe::DEFAULT
// (America/Lima) siempre: si le llega la medianoche UTC del dia 6, la escribe
// como las 19:00 del dia 5 y el documento legal queda con una fecha distinta a
// la registrada. El mediodia deja margen para que ninguna conversion de zona
// pueda mover el dia.
func FechaEmisionLima(f time.Time) time.Time {
	return time.Date(f.Year(), f.Month(), f.Day(), 12, 0, 0, 0, Lima)
}
