-- +goose Up

-- Los identificadores son UUIDv7 (RFC 9562), tipo NATIVO de Postgres y no text.
--
-- 16 bytes contra los 30 que ocupa un ULID como text — medido con
-- pg_column_size, casi la mitad del indice. Ordenan por tiempo igual que un
-- ULID, asi que las inserciones van al final del indice y no en medio.
--
-- Postgres 18 los genera el mismo con uuidv7() y les extrae la fecha con
-- uuid_extract_timestamp(), pero aca los pone la aplicacion: el worker necesita
-- el id ANTES del insert para encolar su job de River en la misma transaccion y
-- para armar la clave de la foto en el almacen.

-- Un restaurante. En el demo no hay usuarios: el token es lo unico que prueba
-- que quien pide es el dueno del borrador.
CREATE TABLE restaurante (
    id                       uuid        PRIMARY KEY,
    nombre                   text        NOT NULL,
    -- Nulable a proposito, al contrario que error o foto_clave: la cadena vacia
    -- chocaria contra el UNIQUE en cuanto hubiera dos restaurantes sin publicar,
    -- y Postgres si permite varios NULL en un indice unico.
    slug                     text        UNIQUE,

    -- Solo el SHA-256 del token. Si la base se filtra, los borradores siguen
    -- siendo inaccesibles.
    token_hash               bytea       NOT NULL,

    -- Publicar es mover este puntero, no copiar la carta. Da re-importar sin
    -- downtime y rollback gratis.
    importacion_publicada_id uuid,

    creado_at                timestamptz NOT NULL DEFAULT now()
);

-- Una subida de carta. Es la unidad de trabajo y la unidad de presupuesto.
CREATE TABLE importacion (
    id                 uuid        PRIMARY KEY,
    restaurante_id     uuid        NOT NULL REFERENCES restaurante(id) ON DELETE CASCADE,
    estado             text        NOT NULL CHECK (estado IN ('leyendo','lista','publicada','fallida')),

    -- Hoy NADIE lee esta columna. Existe desde la primera migracion para que
    -- poner un tope de importaciones por IP sea un COUNT(*) en el handler y no
    -- una migracion sobre datos vivos. El demo corre sin tope a proposito.
    ip                 inet,

    -- Las claves de las fotos que subio el dueno, no las fotos.
    imagenes           jsonb       NOT NULL DEFAULT '[]',

    -- Lo que devolvio el modelo, tal cual, antes de tocarlo. No se lee en el
    -- flujo: es la evidencia para depurar una extraccion mala sin volver a
    -- pagarla, y el insumo de cmd/cartabench.
    carta_cruda        jsonb,

    marcas_revisar     int         NOT NULL DEFAULT 0,
    marcas_confirmar   int         NOT NULL DEFAULT 0,

    -- Micro-dolares. int64, nunca float: $3.00 = 3000000, una foto = 33600.
    -- En centimos de dolar una foto seria 3.36 y volveriamos al float.
    --
    -- Son DOS contadores distintos y no redundantes:
    --   reservado_micros  lo que el guard compara, se incrementa ANTES de gastar
    --   gastado_micros    lo real, lo escribe el Libro DESPUES de la llamada
    presupuesto_micros bigint      NOT NULL,
    reservado_micros   bigint      NOT NULL DEFAULT 0,
    gastado_micros     bigint      NOT NULL DEFAULT 0,

    -- Vacio = sin error. Ver el comentario de foto_origen.
    error              text        NOT NULL DEFAULT '',
    creado_at          timestamptz NOT NULL DEFAULT now(),
    actualizado_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX importacion_ip_creado_idx ON importacion (ip, creado_at);
CREATE INDEX importacion_restaurante_idx ON importacion (restaurante_id);

-- Un plato. Fila propia y no JSONB dentro de la carta porque:
--   1. 60 workers de foto escriben a la vez, cada uno la suya, sin lock comun
--   2. el claim antes de pagar es un UPDATE ... WHERE ... RETURNING atomico
--   3. PATCH /v1/platos/:id necesita un :id que sobreviva a reordenar la carta
CREATE TABLE plato (
    id                  uuid        PRIMARY KEY,
    importacion_id      uuid        NOT NULL REFERENCES importacion(id) ON DELETE CASCADE,

    -- La categoria es una columna, no una tabla: una tabla necesitaria ids
    -- propios que sobrevivan a la reorganizacion, que es el mismo problema de
    -- identidad otra vez y sin necesidad. Se promueve el dia que una categoria
    -- tenga atributos propios (foto, horario, visibilidad).
    categoria           text        NOT NULL,
    orden_categoria     int         NOT NULL,
    orden               int         NOT NULL,

    nombre              text        NOT NULL,
    descripcion         text        NOT NULL DEFAULT '',

    -- Los precios NO se normalizan: nunca se filtra ni se ordena por un precio
    -- individual, siempre se leen y escriben enteros con su plato, y
    -- domain.Precio ya serializa solo.
    precios             jsonb       NOT NULL DEFAULT '[]',

    -- El motivo de revision, vacio si el plato esta limpio. Lo calcula
    -- domain.Plato.Verificar() cruzando el texto impreso contra el numero.
    revisar             text        NOT NULL DEFAULT '',

    foto_estado         text        NOT NULL DEFAULT 'vacia'
                        CHECK (foto_estado IN ('vacia','pendiente','generando','lista','error','sin_presupuesto')),
    -- Vacio en vez de NULL: 'no tiene foto' ya lo dice foto_estado, y cada
    -- columna nulable se convierte en un puntero en todo el codigo que la toca.
    -- plato es la tabla que mas se lee y se escribe; ahi eso se nota.
    foto_origen         text        NOT NULL DEFAULT ''
                        CHECK (foto_origen IN ('', 'ia', 'propia')),

    -- La CLAVE en el almacen, nunca la URL. Migrar a R2 pasa a ser un struct
    -- nuevo mas un rclone copy; con URLs guardadas seria un UPDATE masivo sobre
    -- datos vivos.
    foto_clave          text        NOT NULL DEFAULT '',

    -- foto_intentos topea el gasto por plato; foto_tomada_at es lo que permite
    -- rescatar un plato que quedo en 'generando' porque su worker murio.
    foto_intentos       int         NOT NULL DEFAULT 0,
    foto_tomada_at      timestamptz,
    foto_actualizada_at timestamptz
);

CREATE INDEX plato_orden_idx ON plato (importacion_id, orden_categoria, orden);

-- El claim de foto filtra por (importacion, estado) y cuenta los 'generando' de
-- un restaurante para el techo por restaurante. Sin este indice ese COUNT es un
-- scan por cada job.
CREATE INDEX plato_foto_estado_idx ON plato (importacion_id, foto_estado);

-- Cada llamada a la IA, con lo que costo. Es toda la observabilidad que el demo
-- necesita: SUM(costo_micros) GROUP BY tarea responde el unit economics, que es
-- la unica pregunta que hoy importa.
CREATE TABLE gasto_ia (
    id             bigserial   PRIMARY KEY,

    -- NULL a proposito: los CLIs de laboratorio tambien anotan, y su gasto es
    -- real aunque no pertenezca a ninguna importacion.
    importacion_id uuid        REFERENCES importacion(id) ON DELETE SET NULL,

    tarea          text        NOT NULL,
    modelo         text        NOT NULL,
    tokens_entrada int         NOT NULL DEFAULT 0,
    tokens_salida  int         NOT NULL DEFAULT 0,
    imagenes       int         NOT NULL DEFAULT 0,
    costo_micros   bigint      NOT NULL,
    intentos       int         NOT NULL DEFAULT 1,
    creado_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX gasto_ia_importacion_idx ON gasto_ia (importacion_id);

-- +goose Down
DROP TABLE gasto_ia;
DROP TABLE plato;
DROP TABLE importacion;
DROP TABLE restaurante;
