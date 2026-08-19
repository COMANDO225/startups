-- +goose Up

-- Un restaurante puede ser DOS cosas a la vez, y en Peru es lo normal: la
-- cevicheria del barrio que tambien vende pollo a la brasa. Con un solo tipo,
-- la mitad de su carta salia emplatada como no es.
--
-- La lista va ORDENADA: el primero es el principal y es el que decide cuando un
-- plato no se parece a nada. Por eso jsonb y no un text[]: el orden importa y
-- se lee y escribe entero, nunca por elemento.
ALTER TABLE restaurante ADD COLUMN tipos jsonb NOT NULL DEFAULT '[]';

UPDATE restaurante SET tipos = jsonb_build_array(tipo) WHERE tipo <> '';

ALTER TABLE restaurante DROP COLUMN tipo;

-- +goose Down
ALTER TABLE restaurante ADD COLUMN tipo text NOT NULL DEFAULT 'generico';
UPDATE restaurante SET tipo = COALESCE(tipos->>0, 'generico');
ALTER TABLE restaurante DROP COLUMN tipos;
