-- name: SiguienteCorrelativo :one
-- El UPDATE toma un lock de fila, serializando solo por (tenant, tipo, serie).
UPDATE "series"
   SET "correlativo" = "correlativo" + 1
 WHERE "tenant_id" = $1 AND "tipo_doc" = $2 AND "serie" = $3
RETURNING "correlativo";

-- name: CrearComprobante :exec
INSERT INTO "comprobantes" (
    "id", "tenant_id", "idempotency_key", "tipo_doc", "serie", "correlativo",
    "estado", "payload", "moneda", "importe_total", "fecha_emision",
    "created_at", "updated_at"
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
);

-- name: ComprobantePorID :one
SELECT * FROM "comprobantes"
 WHERE "tenant_id" = $1 AND "id" = $2;

-- name: ComprobantePorIdempotencyKey :one
SELECT * FROM "comprobantes"
 WHERE "tenant_id" = $1 AND "idempotency_key" = $2;

-- name: ListarComprobantes :many
SELECT * FROM "comprobantes"
 WHERE "tenant_id" = $1
 ORDER BY "created_at" DESC
 LIMIT $2 OFFSET $3;

-- name: ContarComprobantes :one
SELECT count(*) FROM "comprobantes" WHERE "tenant_id" = $1;

-- name: TomarComprobante :one
-- Guarda de concurrencia: si River entrega el job dos veces, el segundo UPDATE
-- afecta 0 filas y el worker sale sin reintentar.
--
-- La segunda condicion rescata comprobantes que quedaron en 'procesando' porque
-- el proceso murio a mitad. Sin ella se quedarian ahi para siempre: River
-- reintentaria, Tomar devolveria "ya tomado" y el job se cerraria como exitoso
-- sin haber emitido nada.
UPDATE "comprobantes"
   SET "estado"     = 'procesando',
       "intentos"   = "intentos" + 1,
       "tomado_at"  = now(),
       "updated_at" = now()
 WHERE "id" = $1
   AND (
        "estado" = ANY(@estados_tomables::text[])
        OR ("estado" = 'procesando'
            AND "tomado_at" < now() - make_interval(secs => @timeout_segundos::int))
   )
RETURNING *;

-- name: ActualizarComprobante :exec
UPDATE "comprobantes"
   SET "estado"        = $2,
       "xml"           = $3,
       "cdr"           = $4,
       "ticket"        = $5,
       "codigo_sunat"  = $6,
       "mensaje_sunat" = $7,
       "updated_at"    = now()
 WHERE "id" = $1;

-- name: CrearSerie :exec
INSERT INTO "series" ("tenant_id", "tipo_doc", "serie", "correlativo")
VALUES ($1, $2, $3, $4)
ON CONFLICT ("tenant_id", "tipo_doc", "serie") DO NOTHING;

-- name: EstadoComprobante :one
SELECT "estado" FROM "comprobantes" WHERE "id" = $1;

-- name: ComprobantesHuerfanos :many
-- Red de seguridad: comprobantes que llevan demasiado tiempo sin avanzar. Su job
-- pudo cerrarse por error, descartarse tras agotar reintentos o perderse.
-- No hace falta comprobar si ya hay un job encolado: reencolar es inofensivo
-- porque Tomar solo deja pasar a un worker, y River deduplica por args.
SELECT * FROM "comprobantes"
 WHERE "estado" IN ('pendiente', 'procesando', 'error')
   AND "updated_at" < now() - make_interval(secs => @antiguedad_segundos::int)
   AND "intentos" < @max_intentos::int
 ORDER BY "created_at"
 LIMIT $1;

-- name: ComprobantesRequierenAtencion :many
-- Los que ni River ni el barredor pueden arreglar: agotaron reintentos, o SUNAT
-- respondio algo que exige decision humana. Nadie mas los va a mirar.
SELECT * FROM "comprobantes"
 WHERE "tenant_id" = $1
   AND ("estado" = 'duplicado'
        OR ("estado" IN ('pendiente', 'procesando', 'error') AND "intentos" >= @max_intentos::int))
 ORDER BY "updated_at" DESC
 LIMIT $2;

-- name: MarcarWebhookEnviado :exec
UPDATE "comprobantes" SET "webhook_enviado" = true WHERE "id" = $1;

-- name: ComprobantesSinNotificar :many
-- Red de seguridad del webhook: si el job de notificacion se pierde o se
-- descarta, el cliente nunca se entera de que su comprobante quedo resuelto.
SELECT * FROM "comprobantes"
 WHERE "webhook_enviado" = false
   AND "estado" IN ('aceptado', 'observado', 'rechazado', 'anulado')
   AND "updated_at" < now() - make_interval(secs => @antiguedad_segundos::int)
 ORDER BY "updated_at"
 LIMIT @limite::int;
