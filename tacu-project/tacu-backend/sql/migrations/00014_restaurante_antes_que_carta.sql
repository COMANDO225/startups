-- +goose Up

-- El restaurante existe ANTES que su carta.
--
-- Hasta ahora nacian juntos: un solo POST con el nombre y las fotos, asi que no
-- habia forma de guardar quien eres hasta haber subido la carta. El dueno
-- llegaba a una pantalla que le pedia las dos cosas a la vez y el paso "Tus
-- datos" solo existia despues, cuando ya no servia para nada.
--
-- 'nueva' es ese hueco: el restaurante creado, con su token y su tipo de
-- negocio guardados, y todavia sin hojas que leer.
ALTER TABLE importacion DROP CONSTRAINT importacion_estado_check;
ALTER TABLE importacion ADD CONSTRAINT importacion_estado_check
    CHECK (estado IN ('nueva','leyendo','lista','publicada','fallida'));

-- +goose Down
UPDATE importacion SET estado = 'fallida' WHERE estado = 'nueva';
ALTER TABLE importacion DROP CONSTRAINT importacion_estado_check;
ALTER TABLE importacion ADD CONSTRAINT importacion_estado_check
    CHECK (estado IN ('leyendo','lista','publicada','fallida'));
