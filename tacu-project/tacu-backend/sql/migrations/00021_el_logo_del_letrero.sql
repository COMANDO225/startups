-- +goose Up

-- El logo del negocio y la foto del letrero de la que sale.
--
-- Dos columnas y no una porque son cosas distintas y con vidas distintas:
--
--   logo_clave     lo que se PUBLICA. Puede venir de un archivo que el dueno
--                  subio o de redibujar el letrero. Publico.
--   letrero_clave  la FUENTE: la foto del cartel, con sus reflejos y su
--                  plastico. Se guarda para poder volver a redibujar sin pedirle
--                  otra foto, y para ensenarla al lado del resultado. Privada:
--                  no se publica, solo la ve el dueno en su editor.
--
-- Los dos en RESTAURANTE, como la portada: es identidad del negocio, no
-- contenido de una carta.
ALTER TABLE restaurante
    ADD COLUMN logo_clave    text NOT NULL DEFAULT '',
    ADD COLUMN letrero_clave text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE restaurante DROP COLUMN logo_clave, DROP COLUMN letrero_clave;
