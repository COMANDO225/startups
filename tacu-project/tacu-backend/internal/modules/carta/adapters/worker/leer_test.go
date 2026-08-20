package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/app"
	"tacu-backend/internal/modules/carta/domain"
	"tacu-backend/internal/platform/ai"
)

// EL BUG QUE ESTE TEST EXISTE PARA CAZAR: la pantalla marca como hechas TODAS
// las etapas anteriores al numero que recibe, asi que el orden es el contrato
// entero. Clasificar se emitia despues de guardar mientras su constante estaba
// antes, y la secuencia real salia 0,1,2,4,5,3: la lista daba por reconocido el
// tipo de restaurante mientras todavia estaba ordenando, y despues retrocedia.
//
// Basta con que la secuencia sea estrictamente creciente: cualquier reordenado
// futuro que mueva una llamada sin mover su constante rompe esto.
func TestLasEtapasSeEmitenEnOrdenYSinSaltarNinguna(t *testing.T) {
	repo := &repoDeEtapas{}
	w := NuevoLeerCarta(repo, almacenDeUnaFoto(t), lectorQueDevuelve(), organizadorQueNoToca{},
		conocedorQueNoToca{}, sinAtribuir, slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &river.Job[LeerCartaArgs]{Args: LeerCartaArgs{ImportacionID: id.Nuevo()}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work: %v", err)
	}

	if len(repo.etapas) == 0 {
		t.Fatal("no anoto ninguna etapa")
	}
	for i, e := range repo.etapas {
		if i > 0 && e <= repo.etapas[i-1] {
			t.Fatalf("la etapa %d va detras de la %d: %v", e, repo.etapas[i-1], repo.etapas)
		}
	}
	if ultima := repo.etapas[len(repo.etapas)-1]; ultima != domain.EtapaTerminada {
		t.Errorf("termino en la etapa %d, esperaba %d", ultima, domain.EtapaTerminada)
	}

	// Y ninguna se salta: sin esto, borrar una llamada dejaria una fila de la
	// lista marcada como hecha sin que nadie la hubiera hecho.
	for e := int16(0); e <= domain.EtapaTerminada; e++ {
		if !contiene(repo.etapas, e) {
			t.Errorf("la etapa %d no se emitio nunca: %v", e, repo.etapas)
		}
	}
}

func contiene(es []int16, e int16) bool {
	for _, x := range es {
		if x == e {
			return true
		}
	}
	return false
}

// --- dobles ---

type repoDeEtapas struct{ etapas []int16 }

func (r *repoDeEtapas) MarcarEtapa(_ context.Context, _ id.ID, e int16) error {
	r.etapas = append(r.etapas, e)
	return nil
}

func (r *repoDeEtapas) ClavesDeImagenes(context.Context, id.ID) ([]string, error) {
	return []string{"cartas/x/1.jpg"}, nil
}
func (r *repoDeEtapas) GuardarTipos(context.Context, id.ID, []domain.Tipo) error { return nil }
func (r *repoDeEtapas) GuardarCarta(context.Context, id.ID, domain.Carta, []byte, domain.Marcas) error {
	return nil
}
func (r *repoDeEtapas) BancoDePlatos(context.Context) ([]domain.PlatoTipico, error) {
	return nil, nil
}
func (r *repoDeEtapas) MarcarFallida(context.Context, id.ID, string) error { return nil }

type almacenDeUnaSola struct{ bytes []byte }

func (a almacenDeUnaSola) Leer(context.Context, string) ([]byte, string, error) {
	return a.bytes, "image/jpeg", nil
}

func almacenDeUnaFoto(*testing.T) almacenDeUnaSola {
	return almacenDeUnaSola{bytes: []byte("\xff\xd8\xff no es un jpeg de verdad")}
}

type lectorFijo struct{ carta domain.Carta }

func (l lectorFijo) Ejecutar(context.Context, []ai.Imagen) (*app.Resultado, error) {
	return &app.Resultado{Carta: l.carta}, nil
}

func lectorQueDevuelve() lectorFijo {
	return lectorFijo{carta: domain.Carta{
		Categorias: []domain.Categoria{{
			Nombre: "Ceviches",
			Platos: []domain.Plato{{Nombre: "Ceviche Mixto"}},
		}},
	}}
}

type organizadorQueNoToca struct{}

func (organizadorQueNoToca) Ejecutar(_ context.Context, c domain.Carta) (domain.Carta, ai.Uso, error) {
	return c, ai.Uso{}, nil
}

type conocedorQueNoToca struct{}

func (conocedorQueNoToca) Aplicar(context.Context, *domain.Carta, []domain.PlatoTipico) (int, int, error) {
	return 0, 0, nil
}

func sinAtribuir(ctx context.Context, _ id.ID) context.Context { return ctx }
