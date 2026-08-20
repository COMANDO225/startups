-- +goose Up

-- El estilo del dueno son DOS cosas —en que sirve y sobre que— y cada una se
-- llena de las mismas tres maneras: lo de siempre, un texto, o una foto suya.
--
-- Hasta aqui eran tres entradas desparejas: dos textos con ranura propia en el
-- prompt (recipiente, fondo) y una bolsa `referencias` SIN ROL, que viajaba al
-- modelo como imagenes mudas delante del texto. El modelo tenia que adivinar si
-- esa foto era el plato, la mesa o la comida —y la misma bolsa cargaba ademas
-- las fotos de ejemplo del plato, que dicen justo lo contrario.
--
-- Ahora cada foto tiene columna propia, o sea rol, y por eso el prompt puede
-- nombrarla: "reference image 1 is the exact piece of tableware".

-- La foto que subio el dueno de cada ranura. Claves del almacen, nunca URLs.
ALTER TABLE base_foto ADD COLUMN vajilla_clave text NOT NULL DEFAULT '';
ALTER TABLE base_foto ADD COLUMN fondo_clave   text NOT NULL DEFAULT '';

-- vista_clave siempre fue el dibujo de la VAJILLA. Se renombra porque ahora hay
-- dos ranuras simetricas y un nombre asimetrico para cosas simetricas es la
-- clase de detalle que se paga un ano despues.
ALTER TABLE base_foto RENAME COLUMN vista_clave TO vajilla_vista_clave;
ALTER TABLE base_foto ADD COLUMN fondo_vista_clave text NOT NULL DEFAULT '';

-- La primera referencia que hubiera pasa a ser la foto de la vajilla: es la
-- lectura mas fiel de lo que significaba esa bolsa —"guiate de esta foto"— y en
-- la practica el dueno subia el plato. La segunda se pierde: no hay forma de
-- saber si hablaba del plato o del fondo, y adivinar seria copiar el defecto que
-- esta migracion viene a quitar.
UPDATE base_foto
   SET vajilla_clave = referencias->>0
 WHERE jsonb_typeof(referencias) = 'array'
   AND jsonb_array_length(referencias) > 0
   AND referencias->>0 IS NOT NULL;

ALTER TABLE base_foto DROP COLUMN referencias;

-- +goose Down
ALTER TABLE base_foto ADD COLUMN referencias jsonb NOT NULL DEFAULT '[]';
UPDATE base_foto
   SET referencias = jsonb_build_array(vajilla_clave)
 WHERE vajilla_clave <> '';
ALTER TABLE base_foto DROP COLUMN fondo_vista_clave;
ALTER TABLE base_foto RENAME COLUMN vajilla_vista_clave TO vista_clave;
ALTER TABLE base_foto DROP COLUMN fondo_clave;
ALTER TABLE base_foto DROP COLUMN vajilla_clave;
