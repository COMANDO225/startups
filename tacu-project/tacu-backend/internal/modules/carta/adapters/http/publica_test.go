package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tacu-backend/internal/kernel/dinero"
	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

// GET /v1/r/{slug} se sirve SIN token —el enlace se reparte por WhatsApp— y el
// slug sale del nombre del restaurante, o sea que se adivina escribiendolo.
// Devolvia el mismo DTO que ve el dueno en su editor: las hojas de su carta de
// papel, sus fotos de ejemplo, cuanto lleva gastado y que platos tiene marcados.
func TestLaCartaPublicaNoLlevaNadaPrivado(t *testing.T) {
	imp := cartaConDatosPrivados()

	crudo, err := json.Marshal(aCartaPublicaDTO(imp, func(c string) string { return "/media/" + c }))
	if err != nil {
		t.Fatal(err)
	}
	json := string(crudo)

	// Se comprueba sobre el JSON y no campo a campo a proposito: asi tambien
	// falla el dia que alguien anada un campo al DTO publico sin pensarlo.
	for _, prohibido := range []string{
		"cartas/", "referencias/", "estilo/", // claves privadas
		"gastado", "presupuesto", "por_foto", // lo que le cuesta al dueno
		"revisar", "hoja", "ausente", "foto_ajuste", // el estado de su edicion
		"impreso", "manuscrito", // como leyo la IA su carta
		"mas limon", "S/ 25.oo", "estado", "origen",
	} {
		if strings.Contains(json, prohibido) {
			t.Errorf("la carta publica lleva %q:\n%s", prohibido, json)
		}
	}

	// Y si lleva lo que el comensal necesita.
	for _, hace_falta := range []string{
		"El Rincon Criollo", "Ceviches", "Ceviche mixto", "Pescado y mariscos",
		"Personal", "S/ 25.00",
		"/media/r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/fotos/01a0/ceviche_320.webp",
	} {
		if !strings.Contains(json, hace_falta) {
			t.Errorf("a la carta publica le falta %q:\n%s", hace_falta, json)
		}
	}
}

// Y por HTTP, que es donde estaba el agujero: el endpoint tiene que servir el
// DTO recortado y no el del editor.
func TestEndpointDeCartaPublica(t *testing.T) {
	e := montar(t, true, &lectorFalso{})
	e.publicador.carta = cartaConDatosPrivados()

	resp, cuerpo := e.pedir(t, httptest.NewRequest(http.MethodGet, "/v1/r/el-rincon-criollo", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("estado = %d", resp.StatusCode)
	}
	for _, prohibido := range []string{"cartas/", "referencias/", "gastado", "paginas", "revisar"} {
		if strings.Contains(string(cuerpo), prohibido) {
			t.Errorf("/v1/r/{slug} lleva %q:\n%s", prohibido, cuerpo)
		}
	}
	if !strings.Contains(string(cuerpo), "Ceviche mixto") {
		t.Errorf("no llego la carta:\n%s", cuerpo)
	}
}

func cartaConDatosPrivados() domain.Importacion {
	return domain.Importacion{
		ID:          id.Nuevo(),
		Estado:      domain.Lista,
		Restaurante: domain.Restaurante{Nombre: "El Rincon Criollo", Slug: "el-rincon-criollo"},
		Gastado:     dinero.USD(2.71),
		Presupuesto: dinero.USD(3.00),
		Imagenes: []string{
			"r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/hoja.jpg",
		},
		Carta: domain.Carta{Categorias: []domain.Categoria{{
			Nombre: "Ceviches",
			Platos: []domain.Plato{{
				ID:          id.Nuevo(),
				Nombre:      "Ceviche mixto",
				Descripcion: "Pescado y mariscos",
				Precios: []domain.Precio{{
					Etiqueta: "Personal", Centimos: 2500, Texto: "S/ 25.oo",
					Procedencia: domain.Manuscrito,
				}},
				Foto: domain.Foto{
					Estado: domain.FotoLista, Origen: domain.FotoDeIA,
					Clave: "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/fotos/01a0/ceviche.webp",
				},
				FotoAjuste: "mas limon",
				FotoReferencias: []string{
					"r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/referencias/01a0/suya.jpg",
				},
				Revisar: domain.PrecioDiscordante,
				Hoja:    "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/cartas/01a0/hoja.jpg",
			}},
		}}},
	}
}

// La referencia lleva clave Y url. El frontend borra por la clave: si el DTO
// mandara solo la URL, quitar una foto de ejemplo no daria error, simplemente no
// borraria nada.
func TestLaReferenciaLlevaSuClave(t *testing.T) {
	imp := cartaConDatosPrivados()
	dto := aPlatoDTO(imp.Carta.Categorias[0].Platos[0], func(c string) string { return "/media/" + c + "?firmada" })

	if len(dto.FotoReferencias) != 1 {
		t.Fatalf("referencias = %d", len(dto.FotoReferencias))
	}
	r := dto.FotoReferencias[0]
	if r.Clave != "r/01a0214b-fa16-78dd-bbbe-d6534b1d93a9/referencias/01a0/suya.jpg" {
		t.Errorf("clave = %q, y tiene que ser la del almacen, sin la URL alrededor", r.Clave)
	}
	if r.URL != "/media/"+r.Clave+"?firmada" {
		t.Errorf("url = %q", r.URL)
	}
}
