package app

import (
	"context"
	"fmt"

	"tacu-backend/internal/platform/imagen"
)

// AlmacenDeImagenes es lo minimo que piden estos ayudantes. Estrecha a
// proposito: el worker vive en otro paquete y necesita la MISMA politica de
// variantes, pero no tiene por que saber armar URLs.
type AlmacenDeImagenes interface {
	Guardar(ctx context.Context, clave string, bytes []byte) error
	Borrar(ctx context.Context, clave string) error
}

// guardarFoto normaliza y escribe las tres variantes de una foto de plato o de
// estilo. Devuelve la clave de la grande, que es la que viaja en la base.
//
// EL ORDEN IMPORTA: primero los archivos, y la fila de la base la escribe quien
// llama, despues y solo si esto no fallo. Un objeto huerfano no lo ve nadie; una
// fila que apunta a un archivo que no esta es una imagen rota en el catalogo,
// que es lo que ya nos paso una vez.
func GuardarFoto(ctx context.Context, alm AlmacenDeImagenes, base string, bytes []byte) (string, error) {
	vs, err := imagen.Normalizar(bytes)
	if err != nil {
		return "", err
	}

	clave := base + imagen.Extension
	for _, v := range imagen.Variantes {
		if err := alm.Guardar(ctx, imagen.ConVariante(clave, v), vs[v]); err != nil {
			return "", fmt.Errorf("guardando la variante %q: %w", v, err)
		}
	}
	return clave, nil
}

// guardarHoja escribe una hoja de la carta: la original INTACTA y una miniatura.
//
// La original no se toca porque de ella la IA lee los precios, y reducirla a
// 1280 puede volver ilegible la letra chica de una carta apretada. Es la unica
// imagen del sistema que se guarda tal cual llega.
//
// La miniatura es para el mosaico del editor, que las pinta a 104 px: sin ella
// el dueno se descarga cuatro fotos de teléfono para ver cuatro sellos.
func GuardarHoja(ctx context.Context, alm AlmacenDeImagenes, clave string, bytes []byte) error {
	if err := alm.Guardar(ctx, clave, bytes); err != nil {
		return err
	}

	vs, err := imagen.Normalizar(bytes)
	if err != nil {
		// Una hoja que no se puede reducir sigue sirviendo: la lectura usa la
		// original, y el mosaico se aguanta con ella. No es motivo para
		// rechazar la subida.
		return nil
	}
	return alm.Guardar(ctx, imagen.ConVariante(clave, imagen.Pequena), vs[imagen.Pequena])
}

// borrarFoto se lleva las tres variantes.
//
// Sin esto, cada regeneracion deja la anterior en el bucket para siempre: las
// claves son nuevas en cada escritura —a proposito, por la cache del navegador—
// asi que nada las pisa. En disco no se nota; en R2 es almacenamiento que solo
// crece y se paga todos los meses.
func BorrarFoto(ctx context.Context, alm AlmacenDeImagenes, clave string) error {
	if clave == "" {
		return nil
	}
	for _, v := range imagen.Variantes {
		if err := alm.Borrar(ctx, imagen.ConVariante(clave, v)); err != nil {
			return fmt.Errorf("borrando la variante %q: %w", v, err)
		}
	}
	return nil
}
