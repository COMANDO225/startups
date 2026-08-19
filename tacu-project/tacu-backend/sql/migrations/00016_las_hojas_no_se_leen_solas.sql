-- +goose Up

-- Se va el contador de hojas en vuelo.
--
-- Existia porque anadir una hoja la mandaba a leer sola, en su propio job, y la
-- pantalla necesitaba saber cuantas habia en cola. Ese camino ya no existe: el
-- dueno edita sus hojas —anade, quita, reordena— y despues pulsa "leer mi
-- carta", que dispara UNA lectura sobre la carta entera.
--
-- Eran dos formas distintas de meter platos con reglas distintas: la de la hoja
-- suelta sumaba sin cruzar, y por eso quitar una hoja no deshacia nada de lo que
-- esa hoja habia metido.
ALTER TABLE importacion DROP COLUMN paginas_pendientes;

-- +goose Down
ALTER TABLE importacion ADD COLUMN paginas_pendientes int NOT NULL DEFAULT 0;
