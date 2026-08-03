CREATE TABLE "tenants" (
    "id"                text        PRIMARY KEY,
    "ruc"               text        NOT NULL UNIQUE,
    "razon_social"      text        NOT NULL,
    "nombre_comercial"  text,
    "direccion"         text        NOT NULL,
    "ubigeo"            text        NOT NULL DEFAULT '150101',
    "departamento"      text        NOT NULL DEFAULT 'LIMA',
    "provincia"         text        NOT NULL DEFAULT 'LIMA',
    "distrito"          text        NOT NULL DEFAULT 'LIMA',
    "cert_cifrado"      bytea       NOT NULL,
    "sol_user"          text        NOT NULL,
    "sol_pass_cifrado"  bytea       NOT NULL,
    "produccion"        boolean     NOT NULL DEFAULT false,
    "api_key_hash"      text        NOT NULL,
    "created_at"        timestamptz NOT NULL DEFAULT now(),
    "updated_at"        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX "idx_tenants_api_key_hash" ON "tenants" ("api_key_hash");

-- Contador de correlativos. El incremento ocurre dentro de la transaccion que
-- crea el comprobante: un reintento debe reusar el numero, no generar otro.
CREATE TABLE "series" (
    "tenant_id"   text   NOT NULL REFERENCES "tenants" ("id") ON DELETE CASCADE,
    "tipo_doc"    text   NOT NULL,
    "serie"       text   NOT NULL,
    "correlativo" bigint NOT NULL DEFAULT 0,
    PRIMARY KEY ("tenant_id", "tipo_doc", "serie")
);

CREATE TABLE "comprobantes" (
    "id"              text        PRIMARY KEY,
    "tenant_id"       text        NOT NULL REFERENCES "tenants" ("id"),
    "idempotency_key" text        NOT NULL,
    "tipo_doc"        text        NOT NULL,
    "serie"           text        NOT NULL,
    "correlativo"     bigint      NOT NULL,
    "estado"          text        NOT NULL DEFAULT 'pendiente',
    "payload"         jsonb       NOT NULL,
    "moneda"          text        NOT NULL DEFAULT 'PEN',
    "importe_total"   numeric(12,2) NOT NULL,
    "fecha_emision"   date        NOT NULL,
    "xml"             bytea,
    "cdr"             bytea,
    "ticket"          text,
    "codigo_sunat"    text,
    "mensaje_sunat"   text,
    "intentos"        integer     NOT NULL DEFAULT 0,
    "tomado_at"       timestamptz,
    "created_at"      timestamptz NOT NULL DEFAULT now(),
    "updated_at"      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT "chk_comprobantes_estado" CHECK ("estado" IN (
        'pendiente', 'procesando', 'enviado', 'ticket_pendiente',
        'aceptado', 'observado', 'rechazado', 'error'
    )),

    -- Un retry HTTP del cliente no puede quemar un correlativo.
    CONSTRAINT "uq_comprobantes_idempotency" UNIQUE ("tenant_id", "idempotency_key"),

    -- Hace imposible la doble emision aunque el worker se ejecute dos veces.
    CONSTRAINT "uq_comprobantes_numeracion" UNIQUE ("tenant_id", "tipo_doc", "serie", "correlativo")
);

CREATE INDEX "idx_comprobantes_tenant_estado" ON "comprobantes" ("tenant_id", "estado");
CREATE INDEX "idx_comprobantes_created_at" ON "comprobantes" ("created_at" DESC);
