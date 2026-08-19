package app

import (
	"context"
	"errors"
	"testing"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/modules/carta/domain"
)

type repoPaginasFalso struct {
	imp    domain.Importacion
	fallar error

	// La hoja cuyos platos se marcaron ausentes al quitarla.
	hojaMarcada string
}

func (r *repoPaginasFalso) BorrarPlatosDeLaHoja(_ context.Context, _ id.ID, hoja string) (int, error) {
	r.hojaMarcada = hoja
	return 3, nil
}

func (r *repoPaginasFalso) Obtener(context.Context, id.ID) (domain.Importacion, error) {
	return r.imp, nil
}

func (r *repoPaginasFalso) GuardarImagenes(_ context.Context, _ id.ID, claves []string) error {
	if r.fallar != nil {
		return r.fallar
	}
	r.imp.Imagenes = claves
	return nil
}

func conPaginas(claves ...string) (*PaginasUC, *repoPaginasFalso, *almacenFalso) {
	repo := &repoPaginasFalso{imp: domain.Importacion{
		ID:       id.Nuevo(),
		Estado:   domain.Lista,
		Imagenes: claves,
	}}
	alm := nuevoAlmacen()
	return NuevasPaginas(repo, alm), repo, alm
}

// La clave lleva un id nuevo y no el numero de pagina: quitar la 2 y subir otra
// sobrescribiria el archivo viejo y el dueno veria la hoja equivocada.
func TestLaClaveDeUnaPaginaNuevaNoPisaLaAnterior(t *testing.T) {
	uc, repo, _ := conPaginas("cartas/x/2.jpg")

	claves, err := uc.Agregar(context.Background(), repo.imp.ID, []byte("foto"), "image/jpeg")
	if err != nil {
		t.Fatalf("agregar: %v", err)
	}
	if claves[1] == claves[0] {
		t.Fatalf("la clave nueva pisa la vieja: %v", claves)
	}
}

func TestNoCabenMasDeCuatroPaginas(t *testing.T) {
	uc, repo, _ := conPaginas("1", "2", "3", "4")

	_, err := uc.Agregar(context.Background(), repo.imp.ID, []byte("foto"), "image/jpeg")
	if !errors.Is(err, ErrDemasiadasPaginas) {
		t.Fatalf("err = %v, esperaba ErrDemasiadasPaginas", err)
	}
}

// Anadir mientras la carta se lee no puede pasar: esa lectura esta a punto de
// reescribir los platos, y una hoja que entra a mitad no la ve nadie.
func TestNoSePuedeAnadirMientrasLaCartaSeLee(t *testing.T) {
	uc, repo, _ := conPaginas("1")
	repo.imp.Estado = domain.Leyendo

	_, err := uc.Agregar(context.Background(), repo.imp.ID, []byte("foto"), "image/jpeg")
	if !errors.Is(err, ErrCartaOcupada) {
		t.Fatalf("err = %v, esperaba ErrCartaOcupada", err)
	}
	if len(repo.imp.Imagenes) != 1 {
		t.Errorf("no tenia que guardar la hoja: %v", repo.imp.Imagenes)
	}
}

func TestUnArchivoQueNoEsCartaSeRechaza(t *testing.T) {
	uc, repo, alm := conPaginas()

	_, err := uc.Agregar(context.Background(), repo.imp.ID, []byte("MZ..."), "application/x-msdownload")
	if !errors.Is(err, ErrImagenInvalida) {
		t.Fatalf("err = %v, esperaba ErrImagenInvalida", err)
	}
	if len(alm.guardado) != 0 {
		t.Errorf("no tenia que guardar nada: %v", alm.guardado)
	}
}

func TestQuitarSacaLaPaginaYDejaLasDemas(t *testing.T) {
	uc, repo, _ := conPaginas("a", "b", "c")

	claves, err := uc.Quitar(context.Background(), repo.imp.ID, "b")
	if err != nil {
		t.Fatalf("quitar: %v", err)
	}
	if len(claves) != 2 || claves[0] != "a" || claves[1] != "c" {
		t.Fatalf("claves = %v", claves)
	}
}

func TestQuitarUnaPaginaQueNoEsDeEstaCartaFalla(t *testing.T) {
	uc, repo, _ := conPaginas("a")

	if _, err := uc.Quitar(context.Background(), repo.imp.ID, "de/otra/carta.jpg"); !errors.Is(err, ErrPaginaDesconocida) {
		t.Fatalf("err = %v, esperaba ErrPaginaDesconocida", err)
	}
}

func TestReordenarSoloAceptaLasPaginasQueYaEstaban(t *testing.T) {
	uc, repo, _ := conPaginas("a", "b")

	claves, err := uc.Reordenar(context.Background(), repo.imp.ID, []string{"b", "a"})
	if err != nil || claves[0] != "b" {
		t.Fatalf("claves = %v, err = %v", claves, err)
	}

	// Colar una clave ajena en el orden seria una forma de que la carta de otro
	// restaurante acabara en esta.
	casos := map[string][]string{
		"clave ajena":  {"a", "de/otra/carta.jpg"},
		"una de mas":   {"a", "b", "a"},
		"una de menos": {"a"},
	}
	for nombre, orden := range casos {
		if _, err := uc.Reordenar(context.Background(), repo.imp.ID, orden); !errors.Is(err, ErrPaginaDesconocida) {
			t.Errorf("%s: err = %v, esperaba ErrPaginaDesconocida", nombre, err)
		}
	}
}

// Quitar una hoja Y sus platos es una decision del dueno, y no tiene vuelta
// atras: los platos borrados se llevan sus fotos. Por eso nada automatico llama
// aqui — la otra opcion del aviso, mantenerlos, no toca ningun plato.
func TestQuitarLaHojaConSusPlatosLosBorra(t *testing.T) {
	uc, repo, _ := conPaginas("cartas/x/1.jpg", "cartas/x/2.jpg")

	n, err := uc.QuitarConPlatos(context.Background(), repo.imp.ID, "cartas/x/2.jpg")
	if err != nil {
		t.Fatalf("QuitarConPlatos: %v", err)
	}
	if n != 3 {
		t.Errorf("borro %d platos", n)
	}
	if repo.hojaMarcada != "cartas/x/2.jpg" {
		t.Errorf("borro los de %q", repo.hojaMarcada)
	}
}

// Y quitarla a secas no toca un solo plato.
func TestQuitarLaHojaSolaNoBorraNingunPlato(t *testing.T) {
	uc, repo, _ := conPaginas("cartas/x/1.jpg", "cartas/x/2.jpg")

	if _, err := uc.Quitar(context.Background(), repo.imp.ID, "cartas/x/2.jpg"); err != nil {
		t.Fatalf("Quitar: %v", err)
	}
	if repo.hojaMarcada != "" {
		t.Errorf("toco los platos de %q", repo.hojaMarcada)
	}
}
