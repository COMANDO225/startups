-- name: SiguienteCorrelativoResumen :one
INSERT INTO "series_resumen" ("tenant_id", "tipo", "fecha_ref", "correlativo")
VALUES ($1, $2, $3, 1)
ON CONFLICT ("tenant_id", "tipo", "fecha_ref")
DO UPDATE SET "correlativo" = "series_resumen"."correlativo" + 1
RETURNING "correlativo";

-- name: CrearResumen :exec
INSERT INTO "resumenes" ("id", "tenant_id", "tipo", "fecha_ref", "correlativo", "estado")
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ResumenPorID :one
SELECT * FROM "resumenes" WHERE "id" = $1;

-- name: TomarResumen :one
UPDATE "resumenes"
   SET "estado"     = 'procesando',
       "intentos"   = "intentos" + 1,
       "tomado_at"  = now(),
       "updated_at" = now()
 WHERE "id" = $1
   AND (
        "estado" IN ('pendiente', 'error')
        OR ("estado" = 'procesando'
            AND "tomado_at" < now() - make_interval(secs => @timeout_segundos::int))
   )
RETURNING *;

-- name: ActualizarResumen :exec
UPDATE "resumenes"
   SET "estado"        = $2,
       "ticket"        = $3,
       "xml"           = $4,
       "cdr"           = $5,
       "codigo_sunat"  = $6,
       "mensaje_sunat" = $7,
       "updated_at"    = now()
 WHERE "id" = $1;

-- name: ResumenesEnCurso :many
-- Resumenes que todavia no llegaron a un estado final. Se usa tanto para el
-- polling de tickets como para el rescate de los que quedaron colgados.
SELECT * FROM "resumenes"
 WHERE "estado" IN ('pendiente', 'procesando', 'ticket_pendiente', 'error')
   AND "updated_at" < now() - make_interval(secs => @antiguedad_segundos::int)
   AND "intentos" < @max_intentos::int
 ORDER BY "created_at"
 LIMIT @limite::int;

-- name: TenantsConBoletasPendientes :many
-- Agrupa por emisor y fecha: cada combinacion produce un resumen diario.
SELECT DISTINCT "tenant_id", "fecha_emision"
  FROM "comprobantes"
 WHERE "estado" = 'pendiente_resumen'
   AND "resumen_id" IS NULL
 ORDER BY "fecha_emision", "tenant_id"
 LIMIT @limite::int;

-- name: BoletasParaResumen :many
SELECT * FROM "comprobantes"
 WHERE "tenant_id" = @tenant_id::text
   AND "fecha_emision" = @fecha_emision::date
   AND "estado" = 'pendiente_resumen'
   AND "resumen_id" IS NULL
 ORDER BY "correlativo"
 LIMIT @limite::int;

-- name: AsignarResumen :exec
UPDATE "comprobantes"
   SET "resumen_id" = @resumen_id::text, "updated_at" = now()
 WHERE "id" = ANY(@ids::text[]);

-- name: ResolverComprobantesDeResumen :exec
-- El CDR del resumen resuelve de una vez todas las boletas que iban dentro.
--
-- El filtro por estado NO es defensivo, es obligatorio: una boleta anulada se
-- reasigna al RC de baja, y sin este WHERE el CDR de ESE RC la devolvia a
-- 'aceptado'. Nuestro registro terminaba contradiciendo a SUNAT, que ya la tenia
-- dada de baja. Ningun UPDATE masivo de estado puede pisar un estado terminal
-- que esta operacion no dicto.
UPDATE "comprobantes"
   SET "estado"        = @estado::text,
       "codigo_sunat"  = @codigo_sunat::text,
       "mensaje_sunat" = @mensaje_sunat::text,
       "updated_at"    = now()
 WHERE "resumen_id" = @resumen_id::text
   AND "estado" NOT IN ('anulado', 'rechazado');

-- name: ComprobantesDeResumen :many
SELECT * FROM "comprobantes"
 WHERE "resumen_id" = @resumen_id::text
 ORDER BY "correlativo";

-- name: MarcarComprobanteAnulado :exec
UPDATE "comprobantes"
   SET "estado" = 'anulado', "motivo_baja" = @motivo::text, "updated_at" = now()
 WHERE "id" = @id::text;
