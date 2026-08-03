-- SUNAT responde 1033/2109 cuando ya tiene el comprobante: no es rechazo ni
-- aceptacion limpia, necesita revision humana.
ALTER TABLE "comprobantes" DROP CONSTRAINT "chk_comprobantes_estado";

ALTER TABLE "comprobantes" ADD CONSTRAINT "chk_comprobantes_estado" CHECK ("estado" IN (
    'pendiente', 'procesando', 'enviado', 'ticket_pendiente',
    'aceptado', 'observado', 'rechazado', 'duplicado', 'error'
));
