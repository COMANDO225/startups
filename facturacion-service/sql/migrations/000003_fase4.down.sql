DROP INDEX IF EXISTS "idx_comprobantes_pendientes_resumen";

ALTER TABLE "comprobantes"
    DROP COLUMN IF EXISTS "resumen_id",
    DROP COLUMN IF EXISTS "motivo_baja",
    DROP COLUMN IF EXISTS "webhook_enviado";

DROP TABLE IF EXISTS "series_resumen";
DROP TABLE IF EXISTS "resumenes";

ALTER TABLE "tenants"
    DROP COLUMN IF EXISTS "webhook_url",
    DROP COLUMN IF EXISTS "webhook_secret_cifrado";

ALTER TABLE "comprobantes" DROP CONSTRAINT "chk_comprobantes_estado";
ALTER TABLE "comprobantes" ADD CONSTRAINT "chk_comprobantes_estado" CHECK ("estado" IN (
    'pendiente', 'procesando', 'enviado', 'ticket_pendiente',
    'aceptado', 'observado', 'rechazado', 'duplicado', 'error'
));
