package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"tacu-backend/internal/platform/almacen"
)

// La ruta /media servia CUALQUIER clave a cualquiera: la hoja de la carta de un
// restaurante se abria sin token. Estas dos mitades —firmar al armar la URL,
// comprobar al servirla— tienen que estar de acuerdo, y por eso se prueban
// juntas y no cada una por su lado.
func TestServirMediaExigeFirmaEnLoPrivado(t *testing.T) {
	const (
		hoja    = "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/hoja.jpg"
		ceviche = "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/fotos/01a0/ceviche.webp"
		otra    = "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/otra.jpg"
	)

	disco, err := almacen.NuevoDisco(t.TempDir(), "/media")
	if err != nil {
		t.Fatal(err)
	}
	for _, clave := range []string{hoja, ceviche, otra} {
		if err := disco.Guardar(context.Background(), clave, []byte("unos bytes")); err != nil {
			t.Fatal(err)
		}
	}

	firmante, err := almacen.NuevoFirmante("un-secreto-de-treinta-y-dos-o-mas-caracteres")
	if err != nil {
		t.Fatal(err)
	}
	// La MISMA funcion que recibe el handler en produccion.
	url := firmante.Envolver(disco.URL)

	f := fiber.New()
	f.Get("/media/*", servirMedia(disco, firmante))

	pedir := func(t *testing.T, ruta string) *http.Response {
		t.Helper()
		resp, err := f.Test(httptest.NewRequest(http.MethodGet, ruta, nil),
			fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Fatalf("Test: %v", err)
		}
		t.Cleanup(func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() })
		return resp
	}

	casos := []struct {
		nombre string
		ruta   string
		quiero int
	}{
		{"la hoja de la carta, a pelo", "/media/" + hoja, http.StatusForbidden},
		{"la hoja de la carta, firmada", url(hoja), http.StatusOK},
		{"una foto del catalogo, sin firma", "/media/" + ceviche, http.StatusOK},
		// La firma es de OTRA hoja del mismo restaurante: quien edita su carta
		// recibe firmas validas, y no pueden servirle para pedir lo que quiera.
		{"la hoja con la firma de otra", "/media/" + hoja + queryDe(url(otra)), http.StatusForbidden},
		{"una clave que no existe, firmada", url(hoja + ".falsa"), http.StatusNotFound},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if resp := pedir(t, c.ruta); resp.StatusCode != c.quiero {
				t.Errorf("%s -> %d, esperaba %d", c.ruta, resp.StatusCode, c.quiero)
			}
		})
	}

	// Lo privado no lo guarda una cache compartida; lo publico si, que es de
	// donde sale el egreso gratis del catalogo.
	t.Run("cache", func(t *testing.T) {
		if got := pedir(t, url(hoja)).Header.Get("Cache-Control"); got != "private, max-age=31536000, immutable" {
			t.Errorf("privada: %q", got)
		}
		if got := pedir(t, "/media/"+ceviche).Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Errorf("publica: %q", got)
		}
	})
}

func queryDe(url string) string {
	for i := range url {
		if url[i] == '?' {
			return url[i:]
		}
	}
	return ""
}
