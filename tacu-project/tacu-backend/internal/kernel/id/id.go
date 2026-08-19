// Package id genera identificadores UUIDv7.
//
// POR QUE v7 Y NO ULID. La primera version usaba ULID, copiado del servicio
// hermano sin comparar nada. UUIDv7 gana en todo lo que importa aca:
//
//   - Es un estandar de la IETF (RFC 9562, 2024). ULID es una especificacion en
//     el README de un repositorio.
//   - Postgres tiene tipo nativo: 16 bytes contra los 30 que ocupa un ULID como
//     text. Medido con pg_column_size. En el indice de plato eso es casi la
//     mitad.
//   - Postgres 18 ademas los genera el mismo (uuidv7()) y sabe extraerles la
//     fecha (uuid_extract_timestamp), asi que la base no depende de que el id lo
//     ponga la aplicacion.
//   - Ordenan por tiempo igual que un ULID, que es lo unico que ULID daba de mas
//     frente a un UUIDv4.
//   - pgx lo escribe y lo lee sin codec adicional, porque uuid.UUID es [16]byte.
//
// Se pierde longitud en la URL: 36 caracteres contra 26. Es el unico costo.
//
// CUID2 se descarto: es deliberadamente NO ordenable —su autor considera que la
// ordenabilidad filtra informacion—, tiene longitud variable y no tiene tipo
// nativo en Postgres. Lo que ofrece de mas, generar sin coordinacion, lo da
// UUIDv7 igual.
//
// El id NO es un secreto: los 74 bits aleatorios lo hacen inadivinable, pero el
// prefijo revela cuando se creo. El token del dueno se genera aparte, con
// crypto/rand entero.
package id

import (
	"github.com/google/uuid"
)

// ID es un UUIDv7.
type ID = uuid.UUID

// Nulo es el identificador vacio, el que devuelve Parsear cuando falla.
var Nulo ID

// Nuevo devuelve un identificador nuevo, ordenado por tiempo de creacion.
//
// Es seguro desde varias goroutines sin sincronizacion propia: google/uuid ya la
// tiene dentro. (La version con ULID necesitaba un mutex, y la primera que
// escribi usaba un sync.Pool que rompia la monotonia sin avisar.)
//
// Entra en panico si falla, y eso es correcto: el unico motivo por el que
// NewV7 devuelve error es que crypto/rand no responda, y una maquina sin fuente
// de aleatoriedad no puede atender peticiones — no puede generar tokens, ni
// hacer TLS. Seguir a medias seria peor que caerse.
func Nuevo() ID {
	return uuid.Must(uuid.NewV7())
}

// Parsear valida un identificador que viene de fuera: una URL, un JSON.
//
// Se usa en el borde HTTP para rechazar con 400 sin tocar la base, en vez de
// dejar que la consulta devuelva "0 filas" y confundir "esto no es un id" con
// "esto no existe", que son un 400 y un 404.
func Parsear(s string) (ID, bool) {
	u, err := uuid.Parse(s)
	if err != nil {
		return Nulo, false
	}
	return u, true
}

// Valido dice si una cadena es un identificador bien formado.
func Valido(s string) bool {
	_, ok := Parsear(s)
	return ok
}
