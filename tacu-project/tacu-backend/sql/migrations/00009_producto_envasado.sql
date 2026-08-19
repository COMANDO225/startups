-- +goose Up

-- Un producto ENVASADO no es un plato que el cocinero prepara: es algo que el
-- restaurante COMPRA y revende tal cual. Una gaseosa se fotografia en su botella,
-- con su marca, como en cualquier carta de delivery.
--
-- Hace falta una marca aparte porque la plantilla de la foto termina con "no
-- text and no logo" — una regla FIJA que existe para que no salgan carteles
-- inventados encima de la comida. Sin esta columna, la unica forma de mostrar
-- una Inca Kola seria levantar esa regla para todos.
ALTER TABLE plato_tipico ADD COLUMN envasado boolean NOT NULL DEFAULT false;

-- Solo lo que se vende cerrado. Una limonada, una chicha morada o un cafe los
-- hace la casa: van en vaso y sin marca ninguna.
UPDATE plato_tipico SET envasado = true, actualizado_at = now()
 WHERE clave IN ('gaseosa', 'cerveza');

-- +goose Down
ALTER TABLE plato_tipico DROP COLUMN envasado;
