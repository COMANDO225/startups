package domain_test

import (
	"testing"

	"tacu-backend/internal/modules/carta/domain"
)

func TestSlug(t *testing.T) {
	casos := []struct{ nombre, quiere string }{
		{"La Tribuna del Sur", "la-tribuna-del-sur"},
		{"Pollos Galponcito", "pollos-galponcito"},

		// Tildes y enie: la URL tiene que ser legible, no "cevicher-a".
		{"Cevichería El Ñandú", "cevicheria-el-nandu"},
		{"Pollería El Rincón", "polleria-el-rincon"},
		// Con y sin tilde tienen que dar lo MISMO: si no, el dueno reparte una
		// URL y el buscador indexa otra.
		{"Polleria El Rincon", "polleria-el-rincon"},

		// Signos y espacios de mas: un solo guion, y nunca en los bordes.
		{"  Chifa  Wong  ", "chifa-wong"},
		{"D'Onofrio & Cia.", "d-onofrio-cia"},
		{"---Raro---", "raro"},
		{"Sabor 24/7", "sabor-24-7"},

		// Un nombre que no deja ni una letra utilizable no da URL: el caso lo
		// resuelve quien llama poniendo el id, no inventando aqui.
		{"", ""},
		{"...", ""},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := domain.Slug(c.nombre); got != c.quiere {
				t.Errorf("Slug(%q) = %q, quiero %q", c.nombre, got, c.quiere)
			}
		})
	}
}
