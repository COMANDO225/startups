-- +goose Up

-- Se infiere al leer la carta (domain.InferirTipo) y el dueno lo cambia. Vive en
-- restaurante y no en importacion: reimportar no tiene que repetir la eleccion.
ALTER TABLE restaurante ADD COLUMN tipo text NOT NULL DEFAULT 'generico';

-- El plato y el fondo que elige el dueno. Es tabla y no columnas en restaurante
-- porque la jerarquia tiene dos niveles y las categorias no son tabla:
-- categoria='' es la base general, una fila con categoria='Tríos' la pisa.
--
-- '' significa HEREDA: la herencia es campo por campo, no de fila entera.
CREATE TABLE base_foto (
    id             uuid  PRIMARY KEY,
    restaurante_id uuid  NOT NULL REFERENCES restaurante(id) ON DELETE CASCADE,
    categoria      text  NOT NULL DEFAULT '',

    recipiente     text  NOT NULL DEFAULT '',
    fondo          text  NOT NULL DEFAULT '',
    referencias    jsonb NOT NULL DEFAULT '[]',  -- claves del almacen, nunca URLs

    creado_at      timestamptz NOT NULL DEFAULT now(),
    actualizado_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (restaurante_id, categoria)
);

-- Fotos de ejemplo del plato. Claves, no URLs, por lo mismo que foto_clave.
ALTER TABLE plato ADD COLUMN foto_referencias jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE plato DROP COLUMN foto_referencias;
DROP TABLE base_foto;
ALTER TABLE restaurante DROP COLUMN tipo;
