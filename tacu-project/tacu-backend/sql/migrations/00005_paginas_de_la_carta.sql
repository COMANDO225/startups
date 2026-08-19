-- +goose Up

-- Cuantas paginas se estan leyendo AHORA MISMO.
--
-- Es una columna aparte y no el estado 'leyendo' a proposito: al anadir una
-- pagina a una carta ya leida, poner la importacion en 'leyendo' esconderia los
-- 74 platos que el dueno estaba corrigiendo (Obtener no trae la carta en ese
-- estado) y le bloquearia la seccion entera por una pagina.
ALTER TABLE importacion
    ADD COLUMN paginas_pendientes int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE importacion DROP COLUMN paginas_pendientes;
