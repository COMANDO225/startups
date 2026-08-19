package domain

import (
	"strings"
	"unicode"
)

// Slug convierte el nombre del restaurante en la parte legible de su URL.
//
// Se hace en Go y no en Postgres para poder probarlo: la URL publica es lo que
// el dueno reparte por WhatsApp y no puede cambiar sola.
func Slug(nombre string) string {
	var b strings.Builder
	guion := false

	for _, r := range strings.ToLower(strings.TrimSpace(nombre)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			guion = false
		// Las tildes y la enie se transliteran en vez de perderse: "Polleria El
		// Rincon" y "Pollería El Rincón" tienen que dar la misma URL, y un
		// "cevichería" que quedara como "cevicher-a" seria ilegible.
		case unicode.IsLetter(r):
			if s, ok := transliteracion[r]; ok {
				b.WriteString(s)
				guion = false
			}
		default:
			// Un solo guion por tanda de espacios y signos, y nunca al principio.
			if !guion && b.Len() > 0 {
				b.WriteByte('-')
				guion = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}

var transliteracion = map[rune]string{
	'á': "a", 'à': "a", 'ä': "a", 'â': "a",
	'é': "e", 'è': "e", 'ë': "e", 'ê': "e",
	'í': "i", 'ì': "i", 'ï': "i", 'î': "i",
	'ó': "o", 'ò': "o", 'ö': "o", 'ô': "o",
	'ú': "u", 'ù': "u", 'ü': "u", 'û': "u",
	'ñ': "n", 'ç': "c",
}
