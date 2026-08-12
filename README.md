# startups

Monorepo de productos SaaS para el mercado peruano.

> 👉 **Si vas a continuar el trabajo, lee [`CONTEXT.md`](CONTEXT.md) primero.**
> Ahí está todo lo decidido, lo aprendido y lo que falta.

---

## Proyectos

| Carpeta | Qué es | Estado |
|---|---|---|
| [`facturacion-service/`](facturacion-service/) | Emisión de comprobantes electrónicos SUNAT. **Genérico y sin marca**: lo consumirá Tacu y después otros productos | ✅ Listo para producción, verificado contra SUNAT BETA |
| [`tacu-project/`](tacu-project/) | **Tacu** — SaaS para restaurantes peruanos | 📋 Plan de negocio, sin código |

**Nada está desplegado.** Todo corre en Docker Compose local.

---

## La tesis en tres líneas

Los restaurantes peruanos pagan **25–32 % de comisión** a Rappi y PedidosYa, y el
**70 % de las cevicherías ya contrató repartidores propios** para escapar.
**OlaClick** domina el canal digital pero —en su propia documentación— manda a sus
clientes peruanos a comprar un back office aparte para facturar. **PANCA** y
**Wally** dominan la trastienda pero su canal digital es débil.

**Nadie une las dos cosas.** Eso es Tacu.

⚠️ **Sin validar:** nadie ha hablado todavía con un dueño de restaurante. Ver
[`docs/guion-entrevistas.md`](docs/guion-entrevistas.md).

---

## Arrancar `facturacion-service`

```bash
cd facturacion-service
docker compose up -d

# El certificado de prueba no está versionado; se genera así:
openssl req -x509 -newkey rsa:2048 -keyout k.pem -out c.pem -days 730 -nodes \
  -subj "/C=PE/O=EMPRESA DE PRUEBA SAC/CN=20000000001"
cat k.pem c.pem > certificate.pem && rm k.pem c.pem

# Alta del emisor. Devuelve la api_key una sola vez.
curl -X POST localhost:8080/v1/emisores \
  -H 'Authorization: Bearer token-admin-desarrollo' -H 'Content-Type: application/json' \
  -d "{\"ruc\":\"20000000001\",\"razon_social\":\"EMPRESA DE PRUEBA SAC\",
       \"direccion\":\"AV. PRUEBA 123\",\"sol_user\":\"MODDATOS\",\"sol_pass\":\"moddatos\",
       \"cert_pem_b64\":\"$(base64 -w0 certificate.pem)\"}"

curl -X POST localhost:8080/v1/comprobantes \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Idempotency-Key: venta-001' \
  -H 'Content-Type: application/json' -d @factura.json     # → 202
```

Requisitos: Docker, Go 1.25+, `sqlc` (solo si se tocan las queries).

**Las credenciales de SUNAT BETA son públicas** (`20000000001` / `MODDATOS` /
`moddatos`) y el certificado autofirmado sirve: no hace falta RUC ni empresa para
desarrollar. Detalles en [`facturacion-service/CLAUDE.md`](facturacion-service/CLAUDE.md).

---

## Documentos

| Archivo | Qué tiene |
|---|---|
| [`CONTEXT.md`](CONTEXT.md) | **La memoria.** Empieza aquí |
| [`docs/decisiones.md`](docs/decisiones.md) | Qué verticales se descartaron y con qué evidencia |
| [`docs/guion-entrevistas.md`](docs/guion-entrevistas.md) | Preguntas de campo y criterio de decisión |
| [`tacu-project/README.md`](tacu-project/README.md) | Mercado, competencia y precios de Tacu |
| [`facturacion-service/CLAUDE.md`](facturacion-service/CLAUDE.md) | Memoria técnica: glosario SUNAT, reglas de correctitud, bugs y por qué |
