-- EL CLAIM. Es lo primero que hace el worker y lo que impide pagar dos veces.
--
-- Hace tres cosas en una sola sentencia atomica:
--   1. toma el plato solo si esta disponible (pendiente, con error, o abandonado)
--   2. topea los intentos, porque cada uno cuesta $0.0336
--   3. respeta el techo de fotos en vuelo POR RESTAURANTE
--
-- Devolver 0 filas significa "este no me toca" y el worker se retira sin gastar.
--
-- La rama de 'generando' con foto_tomada_at vieja es la VENTANA DE RESCATE: sin
-- ella, un worker que muere deja el plato en 'generando' para siempre y ninguna
-- capa lo recupera. Es el bug de los 30 minutos que documento el servicio hermano.
--
-- La subconsulta del final es el techo por restaurante. Con 8 workers y techo 4,
-- un restaurante solo usa la mitad de la capacidad y hace sus 60 fotos en ~63 s,
-- y dos a la vez van los dos a full sin esperarse. Si fuera mas bajo, el caso mas
-- comun del demo —un restaurante solo— iria a la mitad de velocidad sin motivo.
--
-- EL TECHO ES DURO, y hace falta un lock para que lo sea.
--
-- La primera version no lo tenia y el techo se filtraba: en READ COMMITTED cada
-- claim ve una instantanea que NO incluye los UPDATE sin confirmar de los otros
-- workers, asi que varios cuentan "hay 3 en vuelo" a la vez y entran todos.
-- MEDIDO con el tope en 4: se observaron 6 generando a la vez.
--
-- Lo cierra ReclamarFotoConLock, que toma un pg_advisory_xact_lock sobre la
-- importacion antes de contar. Serializa los claims de UNA MISMA carta y no toca
-- a las demas: dos restaurantes distintos siguen reclamando en paralelo, que es
-- justo lo que el techo quiere proteger.
--
-- El lock es de transaccion, asi que se suelta solo al confirmar o deshacer. No
-- hay forma de olvidarse de liberarlo.
-- name: ReclamarFoto :one
UPDATE plato p
   SET foto_estado    = 'generando',
       foto_tomada_at = now(),
       foto_intentos  = foto_intentos + 1
 WHERE p.id = @plato_id
   AND p.foto_intentos < @max_intentos::int
   AND (
        p.foto_estado IN ('pendiente', 'error')
        OR (p.foto_estado = 'generando' AND p.foto_tomada_at < now() - @rescate::interval)
       )
   AND (SELECT count(*) FROM plato p2
         WHERE p2.importacion_id = @importacion_id
           AND p2.foto_estado    = 'generando'
           AND p2.id            <> @plato_id
           AND p2.foto_tomada_at > now() - @rescate::interval
       ) < @max_por_restaurante::int
RETURNING p.id, p.nombre, p.descripcion, p.categoria, p.plato_tipico,
          p.foto_ajuste, p.foto_referencias, p.foto_clave;

-- El ajuste del dueno para la foto de UN plato. Lo escribe en el lapiz de la
-- tarjeta y se aplica en la siguiente generacion.
--
-- Guardar y regenerar son DOS pasos separados a proposito: el dueno puede
-- corregir el texto tres veces antes de gastar los $0.0336, y el boton de
-- regenerar que ya existe sirve igual sin duplicar la logica de encolado.
-- name: GuardarAjusteFoto :exec
UPDATE plato SET foto_ajuste = @ajuste WHERE id = @plato_id;

-- LA RESERVA DE PRESUPUESTO. Se cobra ANTES de gastar, no despues.
--
-- Si se comprobara despues, 60 workers concurrentes leerian todos "aun queda" y
-- se pasarian todos a la vez. Aqui la condicion y el incremento son la misma
-- sentencia, asi que el ultimo que cabe es el ultimo que entra.
--
-- Para imagenes el estimado ES el real (precio fijo por imagen), asi que no hace
-- falta reconciliar despues.
-- name: ReservarPresupuesto :one
UPDATE importacion
   SET reservado_micros = reservado_micros + @costo::bigint
 WHERE id = @importacion_id
   AND reservado_micros + @costo::bigint <= presupuesto_micros
RETURNING reservado_micros;

-- name: MarcarFotoLista :exec
UPDATE plato
   SET foto_estado         = 'lista',
       foto_origen         = @origen,
       foto_clave          = @clave,
       foto_actualizada_at = now()
 WHERE id = @plato_id;

-- name: MarcarFotoConEstado :exec
UPDATE plato
   SET foto_estado         = @estado,
       foto_actualizada_at = now()
 WHERE id = @plato_id;

-- Los platos a los que tiene sentido generarles foto.
--
-- Se saltan SIEMPRE los que tienen foto propia: la que subio el dueno es mejor
-- que cualquier cosa que generemos, y pisarla seria destruir trabajo suyo.
--
-- UNA FOTO YA HECHA SE REGENERA SOLO SI LA PIDEN POR SU NOMBRE. Esa es la
-- diferencia entre las dos ramas y no es un detalle:
--
--   solo_estos NULL  ("generar todas")  -> vacia y error. Nunca 'lista', porque
--                    darle dos veces al boton pagaria de nuevo las 60 fotos que
--                    ya estaban.
--   solo_estos lleno ("regenerar este") -> tambien 'lista'. Es una peticion
--                    explicita del dueno sobre UN plato, normalmente porque
--                    acaba de corregir el texto con el lapiz.
--
-- Sin la segunda rama el boton Regenerar era un no-op: respondia 202 con
-- encoladas=0, sin error en ningun log y sin que la foto cambiara nunca.
-- name: PlatosGenerables :many
SELECT p.id
  FROM plato p
 WHERE p.importacion_id = @importacion_id
   AND p.foto_origen <> 'propia'
   AND (
        p.foto_estado IN ('vacia', 'error')
        OR (@solo_estos::uuid[] IS NOT NULL AND p.foto_estado = 'lista')
       )
   AND (@solo_estos::uuid[] IS NULL OR p.id = ANY(@solo_estos::uuid[]))
 ORDER BY p.orden_categoria, p.orden, p.id;

-- Los intentos vuelven a CERO al encolar.
--
-- foto_intentos existe para frenar a un plato que falla en bucle contra la misma
-- pared, no para topear cuantas veces el dueno puede pedir su foto a proposito.
-- Sin este reset, corregir el texto con el lapiz dos veces dejaba el plato en
-- intentos=2 y el claim lo rechazaba en silencio para siempre.
--
-- Lo que si topea el gasto sigue en pie y es el que importa: el presupuesto por
-- importacion, que se reserva antes de cada llamada pagada.
-- name: MarcarFotosPendientes :exec
UPDATE plato
   SET foto_estado = 'pendiente', foto_intentos = 0, foto_actualizada_at = now()
 WHERE id = ANY(@ids::uuid[]);

-- El plato con su importacion, que es lo que el worker necesita para saber a que
-- presupuesto cargarle la foto.
-- name: PlatoParaFoto :one
SELECT p.id, p.importacion_id, p.nombre, p.descripcion, p.foto_estado, p.foto_origen
  FROM plato p
 WHERE p.id = @plato_id;

-- El token pertenece a la IMPORTACION, pero los endpoints de foto reciben un
-- plato. Esto resuelve de quien es antes de comprobar nada.
-- name: ImportacionDePlato :one
SELECT importacion_id FROM plato WHERE id = $1;

-- Toma el lock de esta importacion. Se llama ANTES de ReclamarFoto, dentro de la
-- misma transaccion, y se suelta solo al terminarla.
--
-- hashtextextended da un bigint a partir del uuid; la colision entre dos
-- importaciones distintas solo costaria que sus claims se serialicen entre si,
-- que es una perdida de paralelismo despreciable y nunca un dato incorrecto.
-- name: BloquearImportacion :exec
SELECT pg_advisory_xact_lock(hashtextextended(@importacion_id::text, 0));

-- LA BASE DE UNA CATEGORIA, en una sola consulta.
--
-- Devuelve como mucho dos filas: la general (categoria='') y la de esta
-- categoria si existe. El plegado campo por campo se hace en Go con
-- domain.Receta.Sobre, no aqui: en SQL habria que escribir un COALESCE por
-- columna y el dia que la receta gane una ranura habria que acordarse de esto.
--
-- El ORDER BY pone la general PRIMERO para que quien recorre sepa cual es cual
-- sin comparar cadenas: '' ordena antes que cualquier nombre de categoria.
-- name: BaseDeFoto :many
SELECT categoria,
       recipiente, vajilla_clave, vajilla_vista_clave,
       fondo,      fondo_clave,   fondo_vista_clave
  FROM base_foto
 WHERE restaurante_id = @restaurante_id
   AND categoria IN ('', @categoria::text)
 ORDER BY categoria;

-- GuardarBaseDeFoto escribe la fila ENTERA de una categoria: las dos ranuras con
-- su texto, su foto y su dibujo.
--
-- Entera y no campo a campo porque el dibujo pertenece al texto que lo produjo:
-- un UPDATE que cambiara el texto dejando el dibujo viejo le enseniaria al dueno
-- la vajilla anterior diciendole que es la que acaba de escribir.
-- name: GuardarBaseDeFoto :exec
INSERT INTO base_foto (id, restaurante_id, categoria,
                       recipiente, vajilla_clave, vajilla_vista_clave,
                       fondo,      fondo_clave,   fondo_vista_clave)
VALUES (@id, @restaurante_id, @categoria,
        @recipiente, @vajilla_clave, @vajilla_vista_clave,
        @fondo,      @fondo_clave,   @fondo_vista_clave)
ON CONFLICT (restaurante_id, categoria) DO UPDATE
   SET recipiente          = EXCLUDED.recipiente,
       vajilla_clave       = EXCLUDED.vajilla_clave,
       vajilla_vista_clave = EXCLUDED.vajilla_vista_clave,
       fondo               = EXCLUDED.fondo,
       fondo_clave         = EXCLUDED.fondo_clave,
       fondo_vista_clave   = EXCLUDED.fondo_vista_clave,
       actualizado_at      = now();

-- EL ESTILO PROPIO de una categoria, SIN plegar.
--
-- Al contrario que BaseDeFoto, que pliega la general por debajo: aqui se quiere
-- lo que esta fila tiene escrito y nada mas. Lo usa quien va a ESCRIBIR —subir
-- una foto, dibujar una ranura— porque guardar lo heredado lo convertiria en
-- propio y la categoria dejaria de seguir a la general para siempre.
--
-- Sin fila devuelve cero filas: todavia no hay nada propio.
-- name: EstiloPropio :one
SELECT recipiente, vajilla_clave, vajilla_vista_clave,
       fondo,      fondo_clave,   fondo_vista_clave
  FROM base_foto
 WHERE restaurante_id = @restaurante_id
   AND categoria = @categoria;

-- name: GuardarReferenciasDePlato :exec
UPDATE plato SET foto_referencias = @referencias WHERE id = @plato_id;

-- name: GuardarTiposDeRestaurante :exec
UPDATE restaurante SET tipos = @tipos WHERE id = @id;

-- El restaurante de una importacion, con sus tipos. El worker lo necesita para
-- armar la receta y solo tiene el id del plato.
-- name: RestauranteDeImportacion :one
SELECT r.id, r.tipos
  FROM restaurante r
  JOIN importacion i ON i.restaurante_id = r.id
 WHERE i.id = @importacion_id;
