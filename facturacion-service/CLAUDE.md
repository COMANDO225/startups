# facturacion-service

Servicio de emisión de comprobantes electrónicos SUNAT (Perú). Genérico y sin
marca: lo consume Tacu (restaurantes) y después otros productos.

## Stack

| | |
|---|---|
| API | Go + Fiber **v3.4.0** |
| Cola | **River v0.42.0** (Postgres, sin Redis) |
| DB | Postgres + **sqlc** |
| Motor | PHP 8.3 + **greenter/greenter v5.3.0** en contenedor |

`module facturacion-service` — renombrar con `go mod edit` al crear el repo remoto.

## Glosario

| Sigla | Qué es |
|---|---|
| **CPE** | Comprobante de Pago Electrónico |
| **CDR** | Constancia de Recepción. La respuesta de SUNAT. **Sin CDR el comprobante no vale** |
| **UBL 2.1** | Estándar XML de los comprobantes |
| **SEE** | Sistema de Emisión Electrónica. `SEE-Contribuyente` = emites con tu certificado, directo a SUNAT |
| **OSE / PSE** | Intermediarios autorizados. No los usamos: emitimos por cuenta del contribuyente |
| **RC** | Resumen Diario — así viajan las **boletas** (asíncrono) |
| **RA** | Comunicación de Baja |
| **GRE** | Guía de Remisión Electrónica. Fuera de v0 |
| **Ticket** | ID que devuelve SUNAT en envíos asíncronos; se consulta después para obtener el CDR |
| **Clave SOL** | Credenciales del contribuyente ante SUNAT |

## Reglas de correctitud — no negociables

Camino de dinero y validez legal.

1. **El correlativo se asigna en la transacción que crea el comprobante, nunca en
   el worker.** Un reintento debe reusar el mismo número, no generar otro.
2. **Idempotencia en dos niveles:** `Idempotency-Key` en la API (un timeout de red
   no puede quemar un correlativo) y transición de estado en el worker
   (`UPDATE ... WHERE estado = <esperado>`; 0 filas = alguien más avanzó, salir sin error).
3. **`UNIQUE (tenant_id, tipo_doc, serie, correlativo)`.**
4. **Nunca reimplementar la firma XML-DSig.** La canonicalización C14N debe producir
   bytes idénticos a los que valida SUNAT. Greenter (vía `xmlseclibs`, grado SAML)
   ya está resuelto. Si algún día se porta a Go, Greenter es el **oráculo**: se
   compara el XML canónico byte a byte.
5. **River encola dentro de la transacción** (`InsertTx`). Nunca `Insert` suelto —
   eso reintroduce el dual-write que motivó elegir River sobre Asynq.

## Convenciones de código

- **Comentarios solo donde el *por qué* no es obvio**: rarezas de SUNAT, guardas de
  concurrencia, decisiones contraintuitivas. Nada que repita lo que el código dice.
- **Interfaz solo con 2+ implementaciones o cuando un test la necesita.** Nunca
  "por si acaso".
- **Puertos solo en `domain/port.go`.** Un único lugar.
- **`Reconstruct(params struct)`**, nunca parámetros posicionales largos.
- **Un `_test.go` por command.** El obligatorio: ejecutar dos veces produce un solo
  comprobante.

### Fiber v3 — usar lo que ya trae

| Usar | En vez de |
|---|---|
| `StructValidator` en `fiber.Config` | Middleware de bind + validación manual. Valida solo en cada `Bind()` |
| `c.Bind().Body(&req)` / `.URI()` / `.All()` | Parseo manual |
| `fiber.Query[int](c, "page", 1)`, `fiber.Params[T]`, `fiber.Locals[T]` | Conversión a mano |
| middleware `healthcheck` | Paquete `health` propio |
| middleware `responsetime` | — |
| `c.Context()` → `context.Context` | `c.RequestCtx()` es el fasthttp crudo |

**No usar `app.State()` ni `Services`**: son service locator. La inyección explícita
en `wire_*.go` es más testeable.

## Motor (`engine/`)

Stateless: el certificado y la Clave SOL viajan **por request**. Sin `cert.pem`
montado ni `empresas.json` — eso da multi-tenancy real.

Recibe **JSON estructurado, no XML**: Greenter construye el UBL. El servicio Go se
queda en el dominio.

### Entorno BETA de SUNAT

No requiere certificado registrado; sirve uno autofirmado.

```
RUC       20000000001
Usuario   MODDATOS          → setClaveSOL('20000000001', 'MODDATOS', 'moddatos')
Clave     moddatos
Endpoint  SunatEndpoints::FE_BETA
```

### Aprendizajes del spike

- **`setFormaPago(new FormaPagoContado())` es obligatorio en UBL 2.1.** Sin eso,
  SUNAT responde error **3244** *"Debe consignar la información del tipo de
  transacción"*, que no sugiere en nada la causa real.
- El paquete es **`greenter/greenter`** — `greenter/lite` no existe.
- Alpine necesita **`libzip` en runtime**, no solo `libzip-dev` en build.
- La firma debe quedar dentro de `<ext:UBLExtensions><ext:ExtensionContent>`.
  Greenter lo hace solo; un port a Go tendría que replicarlo (SAML la pone en la raíz).
- El contenedor escribe como root en volúmenes montados. Desaparece en v1: el
  engine devolverá XML y CDR en base64 dentro de la respuesta, sin escribir archivos.

### El engine debe devolver JSON pase lo que pase

PHP imprime warnings y fatales **como HTML dentro del cuerpo de la respuesta**.
Un campo faltante producía `Warning: Undefined array key` mezclado con el JSON, y
el cliente Go reportaba `invalid character '<'` — un mensaje que no dice nada.

`server.php` lo cierra con: `display_errors=0`, `set_error_handler` que lanza
excepciones, `register_shutdown_function` para fatales, y `ob_start()` para que
ninguna salida suelta corrompa la respuesta.

⚠️ **El handler NO debe lanzar en deprecaciones.** Twig 3.12 avisa que el filtro
`spaceless` quedó obsoleto dentro de las plantillas de Greenter; convertir ese
aviso en excepción rompe una emisión perfectamente válida. Se registran y siguen.

Del lado Go, `infra/engine/client.go` lee el cuerpo completo (con límite de 16 MB),
verifica el status HTTP e incluye un extracto del cuerpo en el error. Sin eso, un
fallo del motor es indepurable.

## Cómo correrlo

```bash
docker compose up -d
DATABASE_URL="postgres://facturacion:facturacion@localhost:5433/facturacion?sslmode=disable" \
  go run ./cmd/seed engine/certs/certificate.pem      # crea emisor de prueba + series

curl -X POST localhost:8080/v1/comprobantes \
  -H 'Authorization: Bearer test-api-key' \
  -H 'Idempotency-Key: venta-001' \
  -H 'Content-Type: application/json' -d @factura.json     # → 202
curl localhost:8080/v1/comprobantes/<id> -H 'Authorization: Bearer test-api-key'
```

El certificado de prueba se genera con:
```bash
openssl req -x509 -newkey rsa:2048 -keyout k.pem -out c.pem -days 730 -nodes \
  -subj "/C=PE/O=EMPRESA DE PRUEBA SAC/CN=20000000001" && cat k.pem c.pem > certificate.pem
```

## Anulación: el camino depende de cómo llegó a SUNAT, no del tipo

`command.Anular` distingue tres casos, y confundirlos genera rechazos:

| Situación | Qué se hace |
|---|---|
| **Nunca informado** (`pendiente_resumen`, `pendiente`, `error`) | Se marca `anulado` local. **No se comunica nada** — SUNAT nunca lo vio |
| **Factura aceptada** | **Comunicación de Baja (RA)** |
| **Boleta aceptada** | **Resumen Diario (RC)** con la línea en estado `3` (anular) |

Se rechaza anular lo que ya está `anulado`, lo `rechazado` (SUNAT nunca lo
registró) y lo que sigue en vuelo (`procesando`, `ticket_pendiente`).

**Detalle que importa:** el comprobante pasa a `anulado` **antes** de enviarse.
Eso es lo que hace que `detalleDe` construya la línea del RC con
`DetalleAnular` (3) en vez de `DetalleAdicionar` (1).

`POST /v1/comprobantes/:id/anular` con `{"motivo": "..."}` → `202`.

## Guardas contra entrada maliciosa o inconsistente

**El cliente no puede falsificar nada que importe.** `infra/engine/client.go`
sobrescribe siempre `emisor`, `tipo_doc`, `serie`, `correlativo` y `fecha_emision`
con los valores del servidor, sin importar qué venga en el payload. Verificado:
inyectar `ruc: 99999999999`, `serie: F666`, `correlativo: 9999` produce igual un
comprobante con el RUC del tenant y la numeración real.

**Todo lo rechazable se rechaza antes de tocar el contador** (`command.validar`):
tipo de documento, coherencia serie↔tipo (F para facturas, B para boletas), fecha
futura, payload ilegible e `importe_total` distinto al de `totales.importe_total`.
Un correlativo consumido por un documento inválido deja un hueco que SUNAT observa.

**El importe declarado debe coincidir con el que viaja a SUNAT.** Si difieren, el
registro interno contradice al documento legal. `"118"` y `"118.00"` cuentan como
iguales.

## Códigos de SUNAT: el default es rechazar

`domain.ClasificarCodigo`:

| Código | Estado |
|---|---|
| `0` | aceptado |
| `4xxx` | observado (aceptado con advertencias) |
| **`1033`, `2109`** | **duplicado** — SUNAT ya lo tiene |
| cualquier otro | **rechazado** |

**El default es rechazado a propósito:** dar por bueno un comprobante que SUNAT no
aceptó es mucho peor que marcar como rechazado uno que sí pasó.

`1033`/`2109` = *"el comprobante fue registrado previamente con otros datos"*.
Aparece cuando reenviamos algo que sí llegó a SUNAT pero cuyo CDR no alcanzamos a
guardar. No es rechazo (SUNAT lo tiene) ni aceptación limpia ("con otros datos"),
por eso va a un estado propio con `RequiereRevision() == true`.

## Que ningún comprobante quede huérfano: las tres capas

Esta fue la fuga más cara de encontrar. Un comprobante estuvo **30 minutos en
`procesando` con su job en `completed`**: nadie lo iba a tocar nunca.

**Capa 1 — ventana de rescate.** `TomarComprobante` acepta comprobantes en
`procesando` cuyo `tomado_at` supere `TimeoutProcesando` (5 min), para retomar lo
que dejó un worker muerto.

**Capa 2 — no cerrar el job de algo en vuelo.** La capa 1 sola es inútil: si el
worker responde "ya tomado" y devuelve `nil`, River cierra el job y **nunca vuelve
a evaluar la condición de rescate**. Por eso `Procesar` consulta el estado real:

```
estado final     → nil, el trabajo está hecho, cerrar el job
sigue en vuelo   → ErrEnVuelo → river.JobSnooze(2 min)
```

`reintentoEnVuelo` (2 min) debe ser **menor** que `TimeoutProcesando` (5 min) para
que el job siga vivo cuando se abra la ventana.

**Capa 3 — barrido periódico.** Si el job se descarta tras agotar reintentos o se
pierde, ninguna de las dos capas anteriores aplica. `RescatarWorker` corre cada
5 minutos y cubre **las tres cosas que pueden quedar colgadas**:

| Qué barre | Antigüedad | Por qué importa |
|---|---|---|
| **Comprobantes** | 10 min | Sin esto no llegan nunca a SUNAT |
| **Resúmenes** | 10 min | **Un resumen colgado arrastra a todas sus boletas** |
| **Webhooks** | 30 min | Si River agota los 10 intentos, el cliente nunca se entera |

Para los resúmenes distingue: con ticket → reencola la consulta; sin ticket →
reencola el envío. `InsertOpts` usa `UniqueOpts.ByArgs` para que reencolar algo
que ya está en cola sea un no-op.

> ⚠️ **Corrección a una creencia previa:** se dijo que River hacía innecesario el
> barredor. **Falso.** River elimina el *dual-write*, no la necesidad de
> reconciliar cuando un job se cierra o se descarta.

### El tope que evita el bucle infinito

Un comprobante con payload inválido falla siempre. Sin tope, el barredor lo
reencola eternamente. `MaxIntentos = 10`: por encima de eso el barredor lo ignora
y aparece en **`GET /v1/comprobantes/atencion`**, junto con los `duplicado`.

**Ese endpoint es el único lugar donde un comprobante roto se vuelve visible.**
Si devuelve algo distinto de cero, hay que mirarlo.

### Coherencia de tiempos (validada al arrancar)

```
ENGINE_TIMEOUT (90s) < JobTimeout (3m) ≤ reintentoEnVuelo (2m)* < TimeoutProcesando (5m) < huérfano (10m)
```

`config.validate()` falla al arrancar si se rompe la cadena. Dos razones distintas:

- **`ENGINE_TIMEOUT < TimeoutProcesando`**: si el motor pudiera tardar más que la
  ventana de rescate, otro worker retomaría un comprobante en vuelo y **se
  enviaría dos veces a SUNAT**.
- **`ENGINE_TIMEOUT < JobTimeout`**: ⚠️ **River cancela el contexto del job a los
  60 segundos por defecto**, sin importar el timeout del cliente HTTP. La consulta
  de ticket a SUNAT tarda ~60s y moría cancelada antes de poder reprogramarse.
  Un default oculto que anulaba silenciosamente toda la configuración de timeouts.

## Boletas: el ciclo del Resumen Diario

Una factura se envía sola y SUNAT responde con un CDR. **Una boleta no.** Se firma,
espera, y viaja junto a las demás en un Resumen Diario (RC) que devuelve un
**ticket** — no un CDR.

```
POST boleta → firmar (sin enviar) → pendiente_resumen
                                          ↓  cada 15 min
            agrupa por (tenant, fecha) → RC-YYYYMMDD-N → sendSummary → ticket
                                          ↓  JobSnooze cada 3 min
            consultar ticket → CDR → resuelve el resumen Y todas sus boletas
```

`domain.ViajaPorResumen(tipo, serie)` decide la ruta. **No alcanza el tipo de
documento:** una nota de crédito sobre una boleta (serie `B...`) también viaja por
resumen, mientras que la misma nota sobre una factura (serie `F...`) va sola.

El CDR del resumen resuelve **todas** sus boletas de una sola sentencia
(`ResolverComprobantesDeResumen`).

### Aprendizajes del motor (verificados contra la API real)

- Los métodos son **`setFecGeneracion`/`setFecResumen`**, no `setFechaGeneracion`.
  Se comprobaron con introspección del contenedor en vez de suponerlos.
- **`SummaryDetail::setTotal()` exige `float`.** Los montos viajan como *string*
  desde Go para no perder centavos; la conversión se hace en PHP, en el último
  paso antes de generar el XML.
- ⚠️ **`php -S` atiende una petición a la vez.** Con varios workers de River en
  paralelo, uno bloqueaba a los demás hasta el timeout. Se resuelve con
  `PHP_CLI_SERVER_WORKERS=6`; migrar a php-fpm si sube la carga.
- **SUNAT BETA no resuelve los tickets de resumen**: responde *"Error Fetching
  http headers"* tras ~60s. El sistema lo trata como "pendiente" y reprograma,
  que es el comportamiento correcto. No es un fallo del servicio.

## Webhooks

Sin ellos el `202 Accepted` obliga al cliente a hacer polling.

```
comprobante llega a estado final → encola NotificarArgs → POST al webhook_url
   cabecera X-Facturacion-Signature: sha256=<HMAC del cuerpo con webhook_secret>
```

- **Solo se notifican estados finales.** Avisar de intermedios genera ruido.
- **El aviso se encola, no se envía en línea:** un webhook lento o caído no puede
  marcar como fallida una emisión que SUNAT ya aceptó.
- **Reintentos de River** con backoff, hasta 10 intentos.
- 🔒 **Protección SSRF:** en producción el destino debe ser `https` y se rechazan
  IPs privadas, loopback y link-local. Sin eso, un tenant podría apuntar el
  webhook a la red interna y usar el servicio como proxy.

## Estado

- ✅ **Fase 1 — Spike:** CDR real de SUNAT BETA, `ResponseCode 0`.
- ✅ **Fase 2 — Esqueleto:** Fiber v3.4 + River v0.42 + Postgres + engine HTTP.
      Migraciones embebidas (dominio con golang-migrate, River con rivermigrate).
- ✅ **Fase 3 — Feature `comprobante`:** flujo completo verificado contra BETA.
      `POST → 202 → worker → aceptado` con XML y CDR persistidos.
      Verificado: idempotencia, correlativo sin huecos, resiliencia con el motor
      caído (recupera solo tras reintentos, sin duplicar), rescate de atascados.
- ✅ **Endurecimiento:** 7 bugs encontrados rompiendo cosas a propósito.
- ✅ **Fase 4 — Asíncronos:** boletas por Resumen Diario (verificado: ticket real
      de SUNAT), `ConsultarTicketWorker` con `JobSnooze`, webhooks con HMAC
      (verificado end-to-end), notas de crédito/débito y comunicación de baja.

## Endpoints

```
POST /v1/comprobantes            emitir            → 202 {id, numero, estado}
POST /v1/comprobantes/:id/anular dar de baja       → 202
GET  /v1/comprobantes            listar
GET  /v1/comprobantes/atencion   lo que nadie más va a mirar
GET  /v1/comprobantes/:id        detalle + XML y CDR en base64
GET  /livez  /readyz             salud
```

Cabeceras obligatorias: `Authorization: Bearer <api-key>` e `Idempotency-Key`.

## Workers

| Worker | Disparo | Qué hace |
|---|---|---|
| `comprobante.emitir` | Al crear | Factura → envía. Boleta → firma y espera resumen |
| `comprobante.resumen_diario` | Cada 15 min | Agrupa boletas por tenant+fecha y **crea** el RC |
| `comprobante.enviar_resumen` | Al crear el RC/RA | Lo envía a SUNAT y obtiene el ticket |
| `comprobante.consultar_ticket` | `JobSnooze` 3 min | Consulta el ticket hasta obtener el CDR |
| `comprobante.notificar` | Estado final | Entrega el webhook firmado |
| `comprobante.rescatar` | Cada 5 min | Red de seguridad: comprobantes, resúmenes y webhooks |

**Crear y enviar el resumen están separados a propósito:** el envío es una llamada
de red que puede fallar, y si formara parte de la transacción que crea el resumen,
un fallo dejaría las boletas sin resumen. Así el resumen queda persistido con sus
boletas asignadas, y el envío se reintenta solo.

## Deuda conocida

- **`/atencion` existe pero nadie lo mira.** El listado está; falta que algo avise
  (correo, webhook, métrica) cuando deja de estar vacío.
- **Representación impresa con QR:** requisito legal, sin construir.
- **Alta de tenants:** solo existe `cmd/seed`; falta el endpoint real con carga
  del certificado.
- **Notas de crédito/débito:** el motor las construye (`buildNote`) y el camino
  está, pero **no se probaron contra BETA**.
- **`domain.TimeoutProcesando` está duplicado en `config`** como `timeoutProcesando`
  para que config no dependa de un feature. Si uno cambia, hay que cambiar el otro.
- El seed usa una API key fija (`test-api-key`); falta el alta real de tenants.
- El barrido de huérfanos es global: con muchos tenants convendría particionarlo.
