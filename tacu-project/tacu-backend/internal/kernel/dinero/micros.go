package dinero

import (
	"fmt"
	"math"
)

// MicrosUSD son millonesimas de dolar, y son un tipo APARTE de Centimos a
// proposito.
//
// Las dos son int64 y las dos son dinero, asi que sin tipos distintos nada
// impide sumar el precio de un ceviche —soles peruanos— con lo que costo una
// llamada a Gemini —dolares—, y el compilador no diria nada. Son monedas
// distintas y escalas distintas.
//
// Millonesimas y no centimos porque una foto cuesta $0.0336: en centimos de
// dolar serian 3.36, o sea un float, que es exactamente lo que no queremos
// cerca del dinero. En micros son 33600 exactos.
//
//	$3.00  = 3_000_000
//	$0.0336 =    33_600
//	$0.017  =    17_000
type MicrosUSD int64

// USD convierte dolares a micros. Se usa en el borde, al leer la configuracion.
func USD(d float64) MicrosUSD {
	return MicrosUSD(math.Round(d * 1e6))
}

// String formatea con los decimales que hagan falta para que no se pierda nada.
// Un costo de foto no se puede mostrar como "$0.03": la diferencia entre 0.0336
// y 0.03 son doce centavos por carta.
func (m MicrosUSD) String() string {
	if m == 0 {
		return "$0"
	}
	s := fmt.Sprintf("$%.6f", float64(m)/1e6)
	// Se recortan los ceros de la derecha, pero nunca por debajo de dos
	// decimales: "$2.00" se lee como dinero, "$2." no.
	for len(s) > 4 && s[len(s)-1] == '0' && s[len(s)-3] != '.' {
		s = s[:len(s)-1]
	}
	return s
}

// Dolares devuelve el valor como float SOLO para mostrarlo o para hablar con una
// API que pide float. Nunca para calcular: sumar floats de dinero es como se
// pierden centavos.
func (m MicrosUSD) Dolares() float64 { return float64(m) / 1e6 }
