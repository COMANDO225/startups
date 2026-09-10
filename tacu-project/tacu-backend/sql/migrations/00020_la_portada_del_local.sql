-- +goose Up

-- La foto del local, que encabeza el catalogo publico.
--
-- Va en RESTAURANTE y no en importacion: es identidad del negocio, no contenido
-- de una carta. Releer la carta, anadir hojas o volver a publicar no puede
-- llevarsela por delante.
--
-- Se guarda la CLAVE, nunca la URL, como todas las imagenes del proyecto: la URL
-- la arma quien sirve, y asi mudarse de almacen no reescribe filas.
ALTER TABLE restaurante ADD COLUMN portada_clave text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE restaurante DROP COLUMN portada_clave;
