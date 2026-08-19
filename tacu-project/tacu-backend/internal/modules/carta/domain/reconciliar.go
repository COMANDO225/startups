package domain

import "tacu-backend/internal/kernel/id"

// PlatoExistente es lo minimo de un plato ya guardado que hace falta para
// cruzarlo con una lectura nueva.
type PlatoExistente struct {
	ID     id.ID
	Nombre string

	// Hoja es de que hoja de la carta salio. Vacia cuando no se sabe: filas
	// anteriores a la atribucion, o una lectura que no la dijo.
	Hoja string
}

// PlatoEnCarta es un plato con su sitio en la carta. La posicion no vive en
// Plato porque no es del plato: es de donde esta puesto, y cambia cada vez que
// OrganizarCarta reordena.
type PlatoEnCarta struct {
	Plato
	OrdenCategoria int
	Orden          int
}

// Reconciliacion es el plan de escritura de una lectura contra lo que ya hay.
type Reconciliacion struct {
	// Actualizar son los que ya existian. Llevan su ID de siempre, y ESO es
	// todo el objetivo: la foto cuelga del id del plato, asi que conservarlo es
	// conservar los $0.0336 que costo.
	Actualizar []PlatoEnCarta

	// Insertar son los que no estaban. Van sin ID: se lo pone quien guarda.
	Insertar []PlatoEnCarta

	// Ausentes son los que estaban y esta lectura ya no trajo. NO se borran
	// aqui: se marcan para que el dueno confirme, porque borrar por nuestra
	// cuenta un plato que el ya fotografio es destruir trabajo suyo a partir de
	// una suposicion nuestra.
	Ausentes []id.ID
}

// Reconciliar cruza la carta recien leida con los platos ya guardados.
//
// EXISTE PARA NO BORRAR. Guardar una lectura borraba los platos y los volvia a
// insertar con ids nuevos, asi que una relectura se llevaba por delante las 74
// fotos ya pagadas y las correcciones del dueno. Con esto, lo que sigue en la
// carta sigue siendo el MISMO plato.
//
// El cruce es por nombre normalizado y NO incluye la categoria, al contrario que
// PlatosNuevos: OrganizarCarta renombra las secciones en cada lectura, asi que
// meter la categoria en la clave haria que un cambio de "CEVICHES" a "Ceviches"
// convirtiera la carta entera en platos nuevos — y sin foto.
//
// Que manda en que, cuando un plato ya existia:
//
//   - lo IMPRESO lo manda la lectura: nombre, descripcion, precios, categoria y
//     orden vienen del papel, que es la fuente. Si el dueno cambio un precio en
//     su carta, la relectura tiene que traerlo.
//   - lo SUYO no se toca: la foto, su ajuste y sus fotos de ejemplo no salen del
//     papel, asi que ninguna lectura tiene nada que decir sobre ellos. Los
//     conserva el id.
//
// hojas son las que la carta tiene AHORA, y acotan que puede considerarse
// ausente: solo un plato atribuido a una hoja que se acaba de leer. Un plato
// cuya hoja el dueno quito —y cuyos platos eligio mantener— no lo contradice
// esta lectura, porque esta lectura no miro su hoja. Sin esta regla, mantener
// los productos y releer los devolvia marcados como desaparecidos treinta
// segundos despues de haber dicho que se quedaban.
//
// Lo mismo para el plato sin hoja atribuida: si no sabemos de donde salio, no
// podemos decir que la lectura lo desminti0. Se queda.
func Reconciliar(leida Carta, existentes []PlatoExistente, hojas []string) Reconciliacion {
	deLaCarta := make(map[string]bool, len(hojas))
	for _, h := range hojas {
		deLaCarta[h] = true
	}

	// Cola por nombre: una carta puede repetir un nombre en dos secciones, y sin
	// la cola los dos se llevarian el mismo id.
	porNombre := make(map[string][]PlatoExistente, len(existentes))
	for _, e := range existentes {
		s := Slug(e.Nombre)
		porNombre[s] = append(porNombre[s], e)
	}

	var r Reconciliacion
	usados := make(map[id.ID]bool, len(existentes))

	for iCat, cat := range leida.Categorias {
		for iPlato, p := range cat.Platos {
			// La categoria viaja con el plato: el worker de fotos recibe platos
			// sueltos y sin ella no sabe si es una fuente, un trio o una ronda.
			p.Categoria = cat.Nombre
			puesto := PlatoEnCarta{Plato: p, OrdenCategoria: iCat, Orden: iPlato}

			s := Slug(p.Nombre)
			if cola := porNombre[s]; len(cola) > 0 {
				ya := cola[0]
				porNombre[s] = cola[1:]
				usados[ya.ID] = true
				puesto.ID = ya.ID
				r.Actualizar = append(r.Actualizar, puesto)
				continue
			}
			puesto.ID = id.ID{}
			r.Insertar = append(r.Insertar, puesto)
		}
	}

	for _, e := range existentes {
		if usados[e.ID] {
			continue
		}
		if !deLaCarta[e.Hoja] {
			continue
		}
		r.Ausentes = append(r.Ausentes, e.ID)
	}
	return r
}

// AtribuirHojas traduce el numero de hoja que dijo el modelo a la clave real de
// esa hoja en el almacen.
//
// El modelo solo puede decir "esto estaba en la imagen 2": no conoce nuestras
// claves. Y puede decir un numero que no existe —alucinar un 5 con 2 hojas—, en
// cuyo caso el plato se queda SIN hoja en vez de colgarse de una equivocada:
// una atribucion inventada haria que quitar una hoja borrara platos de otra.
func AtribuirHojas(c *Carta, claves []string) {
	for i := range c.Categorias {
		for j := range c.Categorias[i].Platos {
			p := &c.Categorias[i].Platos[j]
			if p.HojaLeida >= 1 && p.HojaLeida <= len(claves) {
				p.Hoja = claves[p.HojaLeida-1]
			}
		}
	}
}

// SinAusentes devuelve la carta sin los platos que la ultima lectura ya no
// trajo, y sin las categorias que se quedan vacias al quitarlos.
//
// Es lo que ve el cliente en la web publica. En la pantalla del dueno SI salen,
// porque ahi lo que hace falta es que decida.
func (c Carta) SinAusentes() Carta {
	fuera := Carta{Categorias: make([]Categoria, 0, len(c.Categorias))}
	for _, cat := range c.Categorias {
		platos := make([]Plato, 0, len(cat.Platos))
		for _, p := range cat.Platos {
			if !p.Ausente {
				platos = append(platos, p)
			}
		}
		if len(platos) == 0 {
			continue
		}
		cat.Platos = platos
		fuera.Categorias = append(fuera.Categorias, cat)
	}
	return fuera
}
