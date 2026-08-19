-- +goose Up

-- En que va la lectura, para que la pantalla no invente una barra de progreso.
--
-- La lectura son seis etapas de ~10 s en total y el dueno esta mirando. Sin
-- esto solo se puede decir "leyendo" y dibujar una animacion que no significa
-- nada; con esto se le dice que estamos cruzando sus precios contra el texto
-- impreso, que es justo lo que le importa que hagamos.
--
-- Es un ENTERO y no el nombre de la etapa: los textos son de la pantalla y
-- cambian con el idioma, el numero no.
ALTER TABLE importacion ADD COLUMN etapa smallint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE importacion DROP COLUMN etapa;
