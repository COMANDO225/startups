-- +goose Up

-- La vista previa del estilo: el plato o la fuente VACIOS, tal como el dueno
-- los acaba de describir.
--
-- Existe porque hoy el dueno escribe "bandeja de madera oscura" en una caja de
-- texto y no sabe si acerto hasta que genera sesenta fotos y las mira. Una sola
-- imagen del recipiente vacio cuesta $0.0336 y le contesta antes de gastar $2.
--
-- Es la clave del almacen, nunca la URL, igual que plato.foto_clave: la URL la
-- arma quien sirve, asi que mudarse de disco a R2 no reescribe estas filas.
ALTER TABLE base_foto ADD COLUMN vista_clave text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE base_foto DROP COLUMN vista_clave;
