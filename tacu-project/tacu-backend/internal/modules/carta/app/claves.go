package app

import (
	"fmt"

	"tacu-backend/internal/kernel/id"
	"tacu-backend/internal/platform/imagen"
)

// COMO SE LLAMA CADA OBJETO DEL ALMACEN, y el unico sitio que lo sabe.
//
// Antes el formato estaba repetido en siete fmt.Sprintf por cinco archivos. Con
// eso, cambiar el esquema —anadir el tenant, mover una carpeta— era encontrarlos
// todos y no olvidarse de ninguno; y olvidarse de uno no rompe nada visible: la
// imagen se guarda en un sitio y se busca en otro.
//
// EL PRIMER SEGMENTO ES EL RESTAURANTE. Con el, borrar todo lo de un dueno es
// borrar un prefijo, y lo que consume se lee del bucket sin tocar la base. El
// segundo decide el bucket: solo "fotos/" es publico (ver almacen.EsPublica).

func prefijoDe(restaurante id.ID) string {
	return "r/" + restaurante.String() + "/"
}

// ClaveDeFoto: la foto de un plato. Publica — su destino es el catalogo.
//
// Devuelve la BASE, sin extension: quien la guarda le anade la suya al
// normalizar, porque el formato de salida lo decide el paquete imagen.
func ClaveDeFoto(restaurante, plato id.ID) string {
	return fmt.Sprintf("%sfotos/%s/%s", prefijoDe(restaurante), plato, id.Nuevo())
}

// ClaveDeHoja: una hoja de la carta de papel. PRIVADA, y es la que mas importa
// que lo sea: es el menu del negocio de otro, fotografiado por el.
//
// Lleva su extension porque la hoja se guarda tal cual llega —de ella lee los
// precios la IA— y puede ser un JPEG o un PDF.
func ClaveDeHoja(restaurante, importacion id.ID, extension string) string {
	return fmt.Sprintf("%scartas/%s/%s%s", prefijoDe(restaurante), importacion, id.Nuevo(), extension)
}

// ClaveDeEstilo: la vajilla o el fondo del dueno, dibujados o subidos. Privada:
// solo se ven en su editor.
func ClaveDeEstilo(restaurante, importacion id.ID) string {
	return fmt.Sprintf("%sestilo/%s/%s", prefijoDe(restaurante), importacion, id.Nuevo())
}

// ClaveDeReferencia: la foto de ejemplo de UN plato, la que dice "asi se ve mi
// ceviche". Privada.
func ClaveDeReferencia(restaurante, plato id.ID) string {
	return fmt.Sprintf("%sreferencias/%s/%s", prefijoDe(restaurante), plato, id.Nuevo())
}

// PrefijoDeRestaurante es lo que hay que borrar para llevarse todo lo de un
// dueno, en los dos buckets.
func PrefijoDeRestaurante(restaurante id.ID) string {
	return prefijoDe(restaurante)
}

// ConVariante se reexporta para que quien sirve una imagen no tenga que importar
// el paquete imagen solo para armar un sufijo.
func ConVariante(clave string, v imagen.Variante) string {
	return imagen.ConVariante(clave, v)
}
