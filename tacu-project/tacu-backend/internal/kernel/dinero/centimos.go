// Package dinero representa montos como enteros de centimos.
//
// float64 esta PROHIBIDO para dinero en este proyecto: 0.1 + 0.2 != 0.3 y los
// errores se acumulan en silencio hasta que un total no cuadra.
package dinero

import (
	"errors"
	"fmt"
	"strings"
)

// Centimos es un monto en centimos de sol. 1250 = S/ 12.50
type Centimos int64

func (c Centimos) String() string {
	signo := ""
	if c < 0 {
		signo, c = "-", -c
	}
	return fmt.Sprintf("%sS/ %d.%02d", signo, c/100, c%100)
}

// Soles devuelve el monto en formato decimal simple: "12.50"
func (c Centimos) Soles() string {
	signo := ""
	if c < 0 {
		signo, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", signo, c/100, c%100)
}

var (
	ErrVacio     = errors.New("monto vacio")
	ErrIlegible  = errors.New("monto ilegible")
	ErrDemasiado = errors.New("monto fuera de rango razonable")
)

// TopeRazonable: ningun plato de restaurante cuesta mas de S/ 5,000. Un monto
// por encima casi siempre es un error de lectura (dos precios pegados, un
// telefono leido como precio), no un plato caro.
const TopeRazonable = Centimos(500_000)

// Parsear lee un precio tal como aparece impreso en una carta peruana y lo
// convierte a centimos.
//
// Es deliberadamente estricto: su trabajo NO es adivinar, es decir "no se" para
// que el precio caiga a revision humana. Un parser generoso aqui reintroduce
// justo el problema que existe para detectar — que algo invente un numero
// plausible.
//
// Acepta: "S/ 12.50", "S/12.50", "12.50", "12,50", "25", "S/ 1,250.00"
func Parsear(s string) (Centimos, error) {
	limpio := strings.TrimSpace(s)
	if limpio == "" {
		return 0, ErrVacio
	}

	// Quitar el simbolo de moneda en sus variantes: S/, S/., s/, $, PEN
	for _, prefijo := range []string{"S/.", "S/", "s/.", "s/", "PEN", "pen", "$"} {
		if resto, ok := strings.CutPrefix(limpio, prefijo); ok {
			limpio = strings.TrimSpace(resto)
			break
		}
	}

	limpio = normalizarSeparadores(limpio)
	if limpio == "" {
		return 0, ErrIlegible
	}

	entero, decimal, tieneDecimal := strings.Cut(limpio, ".")
	if entero == "" {
		return 0, ErrIlegible
	}

	// A partir de aqui todo debe ser digito. Cualquier otra cosa ("12 a 15",
	// "12/15", "desde 12") es ambigua y va a revision.
	if !soloDigitos(entero) {
		return 0, ErrIlegible
	}

	centimos, err := aEntero(entero)
	if err != nil {
		return 0, err
	}
	centimos *= 100

	if tieneDecimal {
		if !soloDigitos(decimal) {
			return 0, ErrIlegible
		}
		switch len(decimal) {
		case 1:
			d, _ := aEntero(decimal)
			centimos += d * 10
		case 2:
			d, _ := aEntero(decimal)
			centimos += d
		default:
			// Mas de dos decimales en un precio de carta no existe; es otra cosa
			// (un codigo, una fecha) leida como precio.
			return 0, ErrIlegible
		}
	}

	if Centimos(centimos) > TopeRazonable {
		return 0, ErrDemasiado
	}
	return Centimos(centimos), nil
}

// normalizarSeparadores resuelve la ambiguedad de la coma en formatos peruanos.
//
// "1,250.00" -> la coma es separador de miles
// "12,50"    -> la coma es separador decimal
func normalizarSeparadores(s string) string {
	s = strings.ReplaceAll(s, " ", "")

	tienePunto := strings.Contains(s, ".")
	tieneComa := strings.Contains(s, ",")

	switch {
	case tienePunto && tieneComa:
		// Conviven: la coma es de miles y el punto decimal.
		return strings.ReplaceAll(s, ",", "")
	case tieneComa:
		// Solo coma: decimal si deja 1 o 2 digitos detras, miles si deja 3.
		if i := strings.LastIndex(s, ","); len(s)-i-1 <= 2 {
			return strings.Replace(s, ",", ".", 1)
		}
		return strings.ReplaceAll(s, ",", "")
	default:
		return s
	}
}

func soloDigitos(s string) bool {
	return s != "" && strings.TrimLeft(s, "0123456789") == ""
}

func aEntero(s string) (int64, error) {
	var n int64
	for i := 0; i < len(s); i++ {
		n = n*10 + int64(s[i]-'0')
		if n > 1<<40 {
			return 0, ErrDemasiado
		}
	}
	return n, nil
}
