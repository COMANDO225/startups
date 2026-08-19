-- name: CrearRestaurante :exec
INSERT INTO restaurante (id, nombre, token_hash, tipos)
VALUES ($1, $2, $3, $4);

-- Las hojas de la carta llegan DESPUES de crear el restaurante, asi que el
-- estado viaja con ellas: es lo mismo que las pone en cola de lectura.
--
-- El WHERE es la guarda: una carta que ya se esta LEYENDO o que ya se leyo no
-- puede volver al principio por aqui. Para anadir hojas a una carta leida esta
-- GuardarPaginas, que suma en vez de reemplazar.
--
-- 'fallida' SI entra: es el "volver a intentar" del dueno cuando la foto salio
-- movida. Sin eso tendria que crear otro restaurante desde cero y perder el
-- nombre y los tipos que ya escribio, que es justo lo que este flujo vino a
-- arreglar. No hay nada que pisar: una lectura fallida no dejo platos.
-- name: GuardarHojasIniciales :execrows
UPDATE importacion
   SET imagenes = @imagenes, estado = @estado
 WHERE id = @id AND estado IN ('nueva','fallida');

-- name: CrearImportacion :exec
INSERT INTO importacion (id, restaurante_id, estado, ip, imagenes, presupuesto_micros)
VALUES ($1, $2, $3, $4, $5, $6);

-- Volver a leer la carta ENTERA desde sus hojas.
--
-- Solo desde 'lista': una que se esta leyendo ya esta en ello, y una publicada
-- tiene un catalogo en la calle que no se puede vaciar por detras.
--
-- Devuelve las filas tocadas para que el borde distinga "no se puede" de "no
-- existe". El contador de paginas pendientes se pone a cero porque la lectura
-- entera reemplaza los platos: una lectura de hoja suelta en vuelo dejaria de
-- tener sentido igual.
-- name: ReleerCarta :execrows
UPDATE importacion
   SET estado = 'leyendo', etapa = 0, error = ''
 WHERE id = @id AND estado = 'lista';

-- name: ObtenerImportacion :one
SELECT i.*, r.nombre AS restaurante_nombre, r.slug AS restaurante_slug
  FROM importacion i
  JOIN restaurante r ON r.id = i.restaurante_id
 WHERE i.id = $1;

-- Devuelve el hash del token para compararlo, no el token: lo que se guarda es
-- el SHA-256 y la comparacion se hace en Go con subtle.ConstantTimeCompare.
-- name: TokenHashDeImportacion :one
SELECT r.token_hash
  FROM importacion i
  JOIN restaurante r ON r.id = i.restaurante_id
 WHERE i.id = $1;

-- Guardar el resultado de la lectura. carta_cruda es lo que devolvio el modelo
-- tal cual: si manana la extraccion sale mal, se depura sobre esto en vez de
-- volver a pagarla.
-- name: MarcarImportacionLista :exec
UPDATE importacion
   SET estado           = 'lista',
       carta_cruda      = $2,
       marcas_revisar   = $3,
       marcas_confirmar = $4,
       error            = '',
       actualizado_at   = now()
 WHERE id = $1;

-- name: MarcarImportacionFallida :exec
UPDATE importacion
   SET estado         = 'fallida',
       error          = $2,
       actualizado_at = now()
 WHERE id = $1;

-- Borrar antes de insertar hace que reintentar el job de lectura entero sea
-- seguro: no quedan platos de la corrida anterior mezclados con los nuevos.
-- name: BorrarPlatosDeImportacion :exec
DELETE FROM plato WHERE importacion_id = $1;

-- name: InsertarPlato :exec
INSERT INTO plato (
    id, importacion_id, categoria, orden_categoria, orden,
    nombre, descripcion, precios, revisar,
    plato_tipico, tipico_origen, hoja_clave
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- Lo minimo para cruzar una lectura contra lo que ya hay.
-- name: PlatosParaReconciliar :many
SELECT id, nombre, hoja_clave FROM plato WHERE importacion_id = $1;

-- El plato que la lectura VOLVIO a traer.
--
-- Se actualiza lo IMPRESO y nada mas: la foto, su ajuste y las fotos de ejemplo
-- del dueno no salen del papel, asi que ninguna lectura tiene nada que decir
-- sobre ellas. Y no se toca el id, que es de donde cuelga la foto.
--
-- ausente vuelve a false: si estaba marcado como desaparecido y la carta lo trae
-- otra vez, es que sigue ahi.
-- name: ActualizarPlatoLeido :exec
UPDATE plato
   SET categoria       = @categoria,
       orden_categoria = @orden_categoria,
       orden           = @orden,
       nombre          = @nombre,
       descripcion     = @descripcion,
       precios         = @precios,
       revisar         = @revisar,
       plato_tipico    = @plato_tipico,
       tipico_origen   = @tipico_origen,
       hoja_clave      = @hoja_clave,
       ausente         = false
 WHERE id = @id;

-- Los que estaban y esta lectura ya no trajo. Se MARCAN, no se borran: pueden
-- tener una foto pagada detras y la lectura pudo equivocarse.
-- name: MarcarPlatosAusentes :exec
UPDATE plato SET ausente = true WHERE id = ANY(@ids::uuid[]);

-- name: BorrarPlato :exec
DELETE FROM plato WHERE id = @id;

-- name: RecuperarPlato :exec
UPDATE plato SET ausente = false WHERE id = @id;

-- Los platos que salieron de UNA hoja, borrados de verdad.
--
-- Se llega aqui SOLO cuando el dueno lo eligio en el aviso de quitar la hoja, y
-- no tiene vuelta atras: si alguno tenia foto, se va con el. La otra opcion del
-- aviso —mantenerlos— no llama a nada, que es justo por lo que no hay un
-- "borrar por si acaso" en ningun camino automatico.
--
-- Los que no tienen hoja atribuida quedan fuera: no sabemos si eran de esta.
-- name: BorrarPlatosDeLaHoja :execrows
DELETE FROM plato
 WHERE importacion_id = @importacion_id AND hoja_clave = @hoja_clave;

-- El orden es el del catalogo, no el de insercion: lo fija OrganizarCarta y el
-- dueno puede cambiarlo despues.
-- name: ListarPlatos :many
SELECT * FROM plato
 WHERE importacion_id = $1
 ORDER BY orden_categoria, orden, id;

-- Lista flaca para el poll del minuto de las fotos: 60 platos son ~7 KB en vez
-- de la carta entera. El front ya tiene el catalogo, solo parchea las URLs.
-- name: ListarEstadoDeFotos :many
SELECT id, foto_estado, foto_clave, foto_origen
  FROM plato
 WHERE importacion_id = $1
 ORDER BY orden_categoria, orden, id;

-- name: AnotarGastoIA :exec
INSERT INTO gasto_ia (
    importacion_id, tarea, modelo,
    tokens_entrada, tokens_salida, imagenes, costo_micros, intentos
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- Lo real gastado se acumula aparte de lo reservado: reservado_micros es lo que
-- el guard compara ANTES de pagar, gastado_micros es lo que costo DESPUES.
-- name: SumarGastado :exec
UPDATE importacion
   SET gastado_micros = gastado_micros + $2,
       actualizado_at = now()
 WHERE id = $1;

-- El unit economics de una carta, que es la unica pregunta de observabilidad que
-- el demo necesita responder.
-- name: GastoPorTarea :many
SELECT tarea, count(*) AS llamadas, sum(costo_micros)::bigint AS costo_micros
  FROM gasto_ia
 WHERE importacion_id = $1
 GROUP BY tarea
 ORDER BY costo_micros DESC;

-- Las claves de las fotos que subio el dueno. El worker las necesita para releer
-- las imagenes del almacen: cuando el job corre, la peticion HTTP que las traia
-- en memoria ya termino hace rato.
-- name: ClavesDeImagenes :one
SELECT imagenes FROM importacion WHERE id = $1;

-- El plato tras una edicion del dueno: sus precios y la marca recalculada.
--
-- No toca la foto ni el orden: el dueno corrige datos, no reorganiza la carta.
-- name: ActualizarPlato :exec
UPDATE plato
   SET precios = @precios,
       revisar = @revisar
 WHERE id = @plato_id;

-- Los motivos de revision de toda la carta, para recontar las marcas tras una
-- edicion.
--
-- Se recuentan en Go y no con un COUNT(...) FILTER porque QUE motivo bloquea y
-- cual solo pide confirmacion es una regla del dominio (MotivoRevision.Nivel).
-- Escrita en SQL, seria la misma regla en dos sitios y el dia que no coincidan
-- la barra de publicar mentiria.
-- name: MotivosDeImportacion :many
SELECT revisar FROM plato WHERE importacion_id = $1;

-- name: ActualizarMarcas :exec
UPDATE importacion
   SET marcas_revisar   = @revisar,
       marcas_confirmar = @confirmar,
       actualizado_at   = now()
 WHERE id = @id;

-- name: PlatoPorID :one
SELECT * FROM plato WHERE id = $1;

-- PUBLICAR es mover un puntero, no copiar la carta.
--
-- El restaurante apunta a la importacion que se ve en publico. Reimportar y
-- volver a publicar es cambiar este uuid: sin downtime, y volver atras es
-- apuntar a la anterior.
-- name: PublicarImportacion :exec
UPDATE restaurante
   SET slug                     = @slug,
       importacion_publicada_id = @importacion_id
 WHERE id = @restaurante_id;

-- name: MarcarImportacionPublicada :exec
UPDATE importacion
   SET estado = 'publicada', actualizado_at = now()
 WHERE id = $1;

-- El slug ya lo tiene OTRO restaurante. Se comprueba antes de escribir para
-- poder anadir un sufijo en vez de estrellarse contra el UNIQUE.
-- name: SlugTomado :one
SELECT EXISTS (
  SELECT 1 FROM restaurante WHERE slug = @slug AND id <> @restaurante_id
);

-- La carta publica. SIN token: es la que ve el cliente del restaurante.
--
-- Va por slug y salta a la importacion PUBLICADA, no a la ultima: seguir
-- editando un borrador nuevo no puede cambiar lo que el cliente esta viendo.
-- name: ImportacionPublicadaPorSlug :one
SELECT i.id, r.nombre AS restaurante_nombre, r.slug AS restaurante_slug
  FROM restaurante r
  JOIN importacion i ON i.id = r.importacion_publicada_id
 WHERE r.slug = @slug;

-- name: MarcarEtapa :exec
UPDATE importacion SET etapa = @etapa, actualizado_at = now() WHERE id = @id;

-- El slug NO se toca aqui: se recalcula al publicar. Renombrar sin publicar deja
-- la direccion vieja viva, que es justo lo que hay que hacer con una URL que el
-- dueno ya repartio por WhatsApp.
-- name: GuardarNombreDeRestaurante :exec
UPDATE restaurante SET nombre = @nombre WHERE id = @id;

-- Las paginas de la carta. Sirve para las tres cosas que el dueno puede hacer
-- con ellas —anadir, quitar y reordenar—: las tres son la misma lista escrita
-- entera, y separarlas en tres UPDATE distintos seria el mismo SQL tres veces.
-- name: GuardarImagenesDeImportacion :exec
UPDATE importacion SET imagenes = @imagenes, actualizado_at = now() WHERE id = @id;

-- GREATEST(0, ...) porque un decremento de mas dejaria el contador en negativo y
-- la pantalla diria "leyendo" para siempre.

-- name: OrdenesDeCategorias :many
SELECT categoria,
       MIN(orden_categoria)::int AS orden_categoria,
       MAX(orden)::int           AS ultimo
  FROM plato
 WHERE importacion_id = @importacion_id
 GROUP BY categoria;
