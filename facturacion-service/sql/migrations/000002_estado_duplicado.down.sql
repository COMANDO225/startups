UPDATE "comprobantes" SET "estado" = 'rechazado' WHERE "estado" = 'duplicado';

ALTER TABLE "comprobantes" DROP CONSTRAINT "chk_comprobantes_estado";

ALTER TABLE "comprobantes" ADD CONSTRAINT "chk_comprobantes_estado" CHECK ("estado" IN (
    'pendiente', 'procesando', 'enviado', 'ticket_pendiente',
    'aceptado', 'observado', 'rechazado', 'error'
));
