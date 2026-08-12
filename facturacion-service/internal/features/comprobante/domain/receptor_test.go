package domain

import "testing"

func TestRUCValido(t *testing.T) {
	casos := []struct {
		ruc      string
		esperado bool
		porque   string
	}{
		{"20000000001", true, "RUC de pruebas de SUNAT"},
		{"20100070970", true, "RUC real de persona juridica"},
		{"10467793549", true, "RUC de persona natural (10 + DNI)"},
		{"20000000009", false, "digito verificador cambiado"},
		{"20100070971", false, "digito verificador cambiado"},
		{"2000000000", false, "10 digitos"},
		{"200000000012", false, "12 digitos"},
		{"2000000000A", false, "lleva una letra"},
		{"", false, "vacio"},
	}

	for _, c := range casos {
		if got := RUCValido(c.ruc); got != c.esperado {
			t.Errorf("RUCValido(%q) = %v, esperaba %v (%s)", c.ruc, got, c.esperado, c.porque)
		}
	}
}

func TestValidarReceptor(t *testing.T) {
	rucOK := Receptor{TipoDoc: DocRUC, NumDoc: "20000000001", RazonSocial: "CLIENTE SAC"}
	dni := Receptor{TipoDoc: DocDNI, NumDoc: "46778912", RazonSocial: "JUAN PEREZ"}
	sinDoc := Receptor{TipoDoc: DocSinRUC, NumDoc: "-", RazonSocial: "VARIOS"}

	casos := []struct {
		nombre  string
		r       Receptor
		tipo    TipoDoc
		serie   string
		importe string
		valido  bool
	}{
		{"factura a RUC", rucOK, TipoFactura, "F001", "118.00", true},
		{"factura a DNI", dni, TipoFactura, "F001", "118.00", false},
		{"boleta a DNI", dni, TipoBoleta, "B001", "118.00", true},
		{"boleta a consumidor final", sinDoc, TipoBoleta, "B001", "25.00", true},
		{"boleta de S/850 sin documento", sinDoc, TipoBoleta, "B001", "850.00", false},
		{"boleta de S/700 justos sin documento", sinDoc, TipoBoleta, "B001", "700.00", true},
		{"importe ilegible sin documento", sinDoc, TipoBoleta, "B001", "abc", false},

		// Una nota hereda la exigencia del documento que modifica.
		{"nota sobre factura a RUC", rucOK, TipoNotaCredito, "F001", "118.00", true},
		{"nota sobre factura a DNI", dni, TipoNotaCredito, "F001", "118.00", false},
		{"nota sobre boleta a DNI", dni, TipoNotaCredito, "B001", "118.00", true},

		{"DNI de 7 digitos", Receptor{DocDNI, "4677891", "JUAN PEREZ"}, TipoBoleta, "B001", "50.00", false},
		{"DNI con letras", Receptor{DocDNI, "4677891A", "JUAN PEREZ"}, TipoBoleta, "B001", "50.00", false},
		{"pasaporte de largo libre", Receptor{DocPasaporte, "X1234", "TURISTA"}, TipoBoleta, "B001", "50.00", true},
		{"pasaporte vacio", Receptor{DocPasaporte, "", "TURISTA"}, TipoBoleta, "B001", "50.00", false},
		{"tipo fuera del catalogo", Receptor{"9", "12345678", "CLIENTE"}, TipoBoleta, "B001", "50.00", false},
		{"razon social de un caracter", Receptor{DocRUC, "20000000001", "C"}, TipoFactura, "F001", "118.00", false},
		{"razon social solo espacios", Receptor{DocRUC, "20000000001", "    "}, TipoFactura, "F001", "118.00", false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := ValidarReceptor(c.r, c.tipo, c.serie, c.importe)
			if c.valido && err != nil {
				t.Fatalf("rechazo un receptor valido: %v", err)
			}
			if !c.valido && err == nil {
				t.Fatal("acepto un receptor que SUNAT rechazaria")
			}
		})
	}
}

// SUNAT corta en 100 caracteres.
func TestRazonSocialLarga(t *testing.T) {
	larga := make([]byte, 101)
	for i := range larga {
		larga[i] = 'A'
	}

	r := Receptor{TipoDoc: DocRUC, NumDoc: "20000000001", RazonSocial: string(larga)}
	if err := ValidarReceptor(r, TipoFactura, "F001", "118.00"); err == nil {
		t.Fatal("acepto una razon social de 101 caracteres")
	}

	r.RazonSocial = string(larga[:100])
	if err := ValidarReceptor(r, TipoFactura, "F001", "118.00"); err != nil {
		t.Fatalf("rechazo una razon social de 100 caracteres: %v", err)
	}
}
