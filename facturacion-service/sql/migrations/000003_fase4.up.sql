-- Webhooks: el 202 no sirve de nada si el cliente tiene que preguntar
-- "¿ya esta?" cada N segundos.
ALTER TABLE "tenants"
    ADD COLUMN "webhook_url"             text,
    ADD COLUMN "webhook_secret_cifrado"  bytea;

-- Las boletas no viajan solas: se juntan en un Resumen Diario (RC) que SUNAT
-- responde con un ticket, no con un CDR.
CREATE TABLE "resumenes" (
    "id"            text        PRIMARY KEY,
    "tenant_id"     text        NOT NULL REFERENCES "tenants" ("id"),
    "tipo"          text        NOT NULL DEFAULT 'RC',
    "fecha_ref"     date        NOT NULL,
    "correlativo"   bigint      NOT NULL,
    "estado"        text        NOT NULL DEFAULT 'pendiente',
    "ticket"        text,
    "xml"           bytea,
    "cdr"           bytea,
    "codigo_sunat"  text,
    "mensaje_sunat" text,
    "intentos"      integer     NOT NULL DEFAULT 0,
    "tomado_at"     timestamptz,
    "created_at"    timestamptz NOT NULL DEFAULT now(),
    "updated_at"    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT "chk_resumenes_tipo" CHECK ("tipo" IN ('RC', 'RA')),

    CONSTRAINT "chk_resumenes_estado" CHECK ("estado" IN (
        'pendiente', 'procesando', 'ticket_pendiente',
        'aceptado', 'observado', 'rechazado', 'duplicado', 'error'
    )),

    CONSTRAINT "uq_resumenes_numeracion" UNIQUE ("tenant_id", "tipo", "fecha_ref", "correlativo")
);

CREATE INDEX "idx_resumenes_estado" ON "resumenes" ("estado", "updated_at");

-- Contador del RC: uno por tenant, tipo y fecha.
CREATE TABLE "series_resumen" (
    "tenant_id"   text   NOT NULL REFERENCES "tenants" ("id") ON DELETE CASCADE,
    "tipo"        text   NOT NULL,
    "fecha_ref"   date   NOT NULL,
    "correlativo" bigint NOT NULL DEFAULT 0,
    PRIMARY KEY ("tenant_id", "tipo", "fecha_ref")
);

ALTER TABLE "comprobantes"
    ADD COLUMN "resumen_id"      text REFERENCES "resumenes" ("id"),
    ADD COLUMN "motivo_baja"     text,
    ADD COLUMN "webhook_enviado" boolean NOT NULL DEFAULT false;

CREATE INDEX "idx_comprobantes_pendientes_resumen"
    ON "comprobantes" ("tenant_id", "fecha_emision")
    WHERE "estado" = 'pendiente_resumen';

-- pendiente_resumen: la boleta ya tiene su XML firmado pero espera que la
-- recoja un resumen diario. anulado: dado de baja ante SUNAT.
ALTER TABLE "comprobantes" DROP CONSTRAINT "chk_comprobantes_estado";

ALTER TABLE "comprobantes" ADD CONSTRAINT "chk_comprobantes_estado" CHECK ("estado" IN (
    'pendiente', 'procesando', 'pendiente_resumen', 'enviado', 'ticket_pendiente',
    'aceptado', 'observado', 'rechazado', 'duplicado', 'anulado', 'error'
));
