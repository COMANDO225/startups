-- name: TenantPorID :one
SELECT * FROM "tenants" WHERE "id" = $1;

-- name: TenantPorAPIKeyHash :one
SELECT * FROM "tenants" WHERE "api_key_hash" = $1;

-- name: CrearTenant :exec
INSERT INTO "tenants" (
    "id", "ruc", "razon_social", "nombre_comercial", "direccion",
    "ubigeo", "departamento", "provincia", "distrito",
    "cert_cifrado", "sol_user", "sol_pass_cifrado", "produccion", "api_key_hash",
    "webhook_url", "webhook_secret_cifrado"
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
);
