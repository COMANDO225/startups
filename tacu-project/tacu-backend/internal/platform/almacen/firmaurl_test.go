package almacen

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

const secretoFirmaURL = "un-secreto-de-treinta-y-dos-o-mas-caracteres"

// claves de las dos clases, con el prefijo de tenant que llevan de verdad.
const (
	privada = "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/hoja.jpg"
	publica = "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/fotos/01a0/ceviche.webp"
)

func nuevoParaTest(t *testing.T) *Firmante {
	t.Helper()
	f, err := NuevoFirmante(secretoFirmaURL)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSecretoCorto(t *testing.T) {
	for _, s := range []string{"", "corto", "0123456789012345678901234567890"} {
		if _, err := NuevoFirmante(s); err == nil {
			t.Errorf("un secreto de %d caracteres tenia que rechazarse", len(s))
		}
	}
	if _, err := NuevoFirmante("01234567890123456789012345678901"); err != nil {
		t.Errorf("32 caracteres tenian que valer: %v", err)
	}
}

func TestFirmarYComprobar(t *testing.T) {
	f := nuevoParaTest(t)
	ahora := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)

	firmada := f.Firmar("/media/"+privada, privada, ahora)
	u, err := url.Parse(firmada)
	if err != nil {
		t.Fatalf("la URL firmada no se puede parsear: %v", err)
	}
	if u.Path != "/media/"+privada {
		t.Errorf("la ruta cambio: %q", u.Path)
	}

	q := u.Query()
	if err := f.Comprobar(privada, q.Get("exp"), q.Get("f"), ahora); err != nil {
		t.Fatalf("la firma recien hecha no vale: %v", err)
	}
}

// Lo publico no se firma ni se exige: el catalogo se abre desde WhatsApp.
func TestLoPublicoPasaSinFirma(t *testing.T) {
	f := nuevoParaTest(t)
	ahora := time.Now()

	if got := f.Firmar("/media/"+publica, publica, ahora); got != "/media/"+publica {
		t.Errorf("una clave publica no se firma, y salio %q", got)
	}
	if err := f.Comprobar(publica, "", "", ahora); err != nil {
		t.Errorf("una clave publica se sirve sin firma: %v", err)
	}
	// Con un dominio propio, URL() ya devuelve la del CDN. Firmarla la romperia.
	cdn := "https://img.tacu.pe/" + publica
	if got := f.Firmar(cdn, publica, ahora); got != cdn {
		t.Errorf("la URL del CDN se toco: %q", got)
	}
}

func TestComprobarRechaza(t *testing.T) {
	f := nuevoParaTest(t)
	ahora := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	q := func(clave string, en time.Time) url.Values {
		u, _ := url.Parse(f.Firmar("/media/"+clave, clave, en))
		return u.Query()
	}
	buena := q(privada, ahora)

	otro, err := NuevoFirmante("otro-secreto-igual-de-largo-que-el-primero")
	if err != nil {
		t.Fatal(err)
	}
	deOtro, _ := url.Parse(otro.Firmar("/media/"+privada, privada, ahora))

	casos := []struct {
		nombre            string
		clave, exp, firma string
		quiero            error
	}{
		{"sin nada", privada, "", "", ErrSinFirma},
		{"sin firma", privada, buena.Get("exp"), "", ErrSinFirma},
		{"sin caducidad", privada, "", buena.Get("f"), ErrSinFirma},
		{"caducidad que no es un numero", privada, "manana", buena.Get("f"), ErrFirmaMala},
		{"firma inventada", privada, buena.Get("exp"), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", ErrFirmaMala},
		// La firma es buena, pero para OTRA imagen: es el ataque que importa,
		// porque quien edita su carta recibe firmas validas de lo suyo.
		{"firma de otra clave", "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/otra.jpg",
			buena.Get("exp"), buena.Get("f"), ErrFirmaMala},
		{"caducidad estirada", privada, "99999999999", buena.Get("f"), ErrFirmaMala},
		{"firmada con otro secreto", privada, deOtro.Query().Get("exp"), deOtro.Query().Get("f"), ErrFirmaMala},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if err := f.Comprobar(c.clave, c.exp, c.firma, ahora); !errors.Is(err, c.quiero) {
				t.Errorf("quiero %v y salio %v", c.quiero, err)
			}
		})
	}
}

func TestCaduca(t *testing.T) {
	f := nuevoParaTest(t)
	ahora := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	u, _ := url.Parse(f.Firmar("/media/"+privada, privada, ahora))
	q := u.Query()

	// Al filo: una firma hecha al final de una ventana tiene que aguantar las 12
	// horas que promete. Si el redondeo se hiciera con una sola ventana en vez
	// de dos, esta seria la que caduca antes de tiempo.
	if err := f.Comprobar(privada, q.Get("exp"), q.Get("f"), ahora.Add(12*time.Hour)); err != nil {
		t.Errorf("a las 12 h todavia tenia que valer: %v", err)
	}
	if err := f.Comprobar(privada, q.Get("exp"), q.Get("f"), ahora.Add(25*time.Hour)); !errors.Is(err, ErrCaducada) {
		t.Errorf("a las 25 h tenia que estar caducada y salio %v", err)
	}
}

// La misma imagen dentro de la misma ventana da la MISMA URL. Es lo que hace que
// el navegador reuse su cache en vez de rebajarse las hojas en cada carga.
func TestLaURLNoSeMueveDentroDeLaVentana(t *testing.T) {
	f := nuevoParaTest(t)
	base := time.Date(2026, 8, 20, 0, 0, 1, 0, time.UTC)

	primera := f.Firmar("/media/"+privada, privada, base)
	for _, d := range []time.Duration{time.Second, time.Hour, 11 * time.Hour} {
		if got := f.Firmar("/media/"+privada, privada, base.Add(d)); got != primera {
			t.Errorf("a los %v la URL cambio:\n  %s\n  %s", d, primera, got)
		}
	}
	if got := f.Firmar("/media/"+privada, privada, base.Add(13*time.Hour)); got == primera {
		t.Error("pasada la ventana la URL tenia que renovarse")
	}
}
