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

# 1. Certificado de prueba (autofirmado: BETA no exige uno registrado)
openssl req -x509 -newkey rsa:2048 -keyout k.pem -out c.pem -days 730 -nodes \
  -subj "/C=PE/O=EMPRESA DE PRUEBA SAC/CN=20000000001"
cat k.pem c.pem > certificate.pem && rm k.pem c.pem

# 2. Alta del emisor por API. Devuelve la api_key UNA sola vez.
curl -X POST localhost:8080/v1/emisores \
  -H 'Authorization: Bearer token-admin-desarrollo' -H 'Content-Type: application/json' \
  -d "{\"ruc\":\"20000000001\",\"razon_social\":\"EMPRESA DE PRUEBA SAC\",
       \"direccion\":\"AV. PRUEBA 123\",\"sol_user\":\"MODDATOS\",\"sol_pass\":\"moddatos\",
       \"cert_pem_b64\":\"$(base64 -w0 certificate.pem)\"}"

# 3. Emitir
curl -X POST localhost:8080/v1/comprobantes \
  -H "Authorization: Bearer $API_KEY" -H 'Idempotency-Key: venta-001' \
  -H 'Content-Type: application/json' -d @factura.json     # → 202

curl "localhost:8080/v1/comprobantes/$ID"         -H "Authorization: Bearer $API_KEY"
curl "localhost:8080/v1/comprobantes/$ID/impresa" -H "Authorization: Bearer $API_KEY"
```

`cmd/seed` sigue existiendo para levantar un entorno rápido desde la terminal, pero
**el camino real es `POST /v1/emisores`**: valida el RUC y el certificado antes de
guardarlo.

Tests: `go test ./...` (los de integración levantan un Postgres con testcontainers;
`-short` los omite).

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
| **no numérico** (`HTTP`, `SOAP-ENV:*`, vacío) | **error** — fallo de transporte, reintentable |
| `0` | aceptado |
| `4xxx` | observado (aceptado con advertencias) |
| **`1033`, `2109`** | **duplicado** — SUNAT ya lo tiene |
| cualquier otro | **rechazado** |

**El default es rechazado a propósito:** dar por bueno un comprobante que SUNAT no
aceptó es mucho peor que marcar como rechazado uno que sí pasó.

### Pero un código no numérico NO viene de SUNAT

Greenter extrae los dígitos del `SoapFault` (`preg_replace('/[^0-9]+/', '', $code)`
en `BaseSunat::getErrorByCode`) y solo devuelve el código crudo cuando no encontró
ninguno: `"HTTP"` en un 401, `"SOAP-ENV:Server"` en una caída.

Eso es un **fallo de transporte, no un rechazo del documento.** Clasificarlo como
rechazado — que es lo que hacía — mataba el comprobante en un estado final y
quemaba el correlativo por un problema de red.

> **Se descubrió con una prueba de carga: 30 emisiones simultáneas → 20 muertas
> con `HTTP Unauthorized`.** Ninguna era un rechazo real.

`procesar.go` y `consultar_ticket.go` devuelven error cuando el estado queda en
`error`, para que River reintente con backoff en vez de cerrar el job.

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
- ✅ **Fase 5 — Producción.** Lo que destapó una prueba de carga de 30 emisiones
      concurrentes (que dejó 30 de 31 rechazadas) y cómo quedó:

| Hallazgo | Antes | Ahora |
|---|---|---|
| Fallo de red clasificado como rechazo | 20 comprobantes muertos, correlativos quemados | `error` retomable; **se recuperan solos en 15–30 s** |
| Sin límite de concurrencia hacia SUNAT | 20 × `401` en una ráfaga | serializado por RUC; **30/30 aceptadas** |
| Validación previa incompleta | 10 × `2022`, correlativo quemado igual | receptor, notas y certificado validados **antes del contador** |
| Fecha del XML ≠ fecha registrada | `IssueDate` un día atrás | mediodía de Lima; **coinciden** |
| Sin representación impresa | requisito legal sin cumplir | QR + HTML imprimible, **QR decodificado y verificado** |
| Notas nunca enviadas | camino sin ejercitar | **crédito y débito aceptadas por SUNAT** |
| `php -S` (servidor de desarrollo) | expuesto al host, sin supervisión | **FrankenPHP**, sin puerto publicado |
| Alta de emisores | solo `cmd/seed` | `POST /v1/emisores`, verificado desde base vacía |
| Sin observabilidad | a ciegas | `/metrics` + alerta cuando `/atencion` deja de estar vacío |
| 21 tests, 4 casos de uso sin ninguno | fakes simulaban los constraints | **65 tests**, integración con Postgres real (testcontainers) |

## Endpoints

```
POST /v1/emisores                 alta de emisor    → 201 {api_key, ...}   [ADMIN_TOKEN]
POST /v1/comprobantes             emitir            → 202 {id, numero, estado}
POST /v1/comprobantes/:id/anular  dar de baja       → 202
GET  /v1/comprobantes             listar
GET  /v1/comprobantes/atencion    lo que nadie más va a mirar
GET  /v1/comprobantes/:id         detalle + XML, CDR, qr y hash
GET  /v1/comprobantes/:id/impresa representación impresa (HTML para imprimir)
GET  /v1/comprobantes/:id/qr.png  QR suelto, para impresora térmica
GET  /metrics                     formato Prometheus  [METRICS_TOKEN, opcional]
GET  /livez  /readyz              salud
```

Cabeceras obligatorias: `Authorization: Bearer <api-key>` e `Idempotency-Key`.

`POST /v1/emisores` usa **su propio token** (`ADMIN_TOKEN`), no una API key: es el
endpoint que las crea. Sin el token configurado, el alta queda deshabilitada.
La `api_key` se muestra **una sola vez**; en la base solo queda su SHA-256.

### En `sol_user` va un usuario SECUNDARIO, no la Clave SOL principal

Con la Clave SOL principal se puede declarar impuestos, ver toda la información
tributaria del cliente y tocar su RUC. Con un **usuario secundario** de perfil
restringido, solo emitir comprobantes.

El WSSE autentica igual (`RUC + usuario`), así que **el código no cambia**: es una
regla de onboarding. Lo crea el cliente desde su SOL; nosotros no podemos.

> El certificado es otra historia: firma cualquier cosa y **eso no se puede acotar**
> en esta arquitectura. O vive aquí, o no hay SaaS. Acotarlo de verdad exige un HSM,
> y eso es para cuando haya volumen.

Detalle legal completo (PSE vs OSE vs SEE-Del Contribuyente) en `../CONTEXT.md`.

## Representación impresa y QR

Requisito legal desde enero 2019. Cadena del QR, separada por `|`:

```
RUC | TIPO_DOC | SERIE | NUMERO | IGV | TOTAL | FECHA | TIPO_DOC_ADQ | NUM_DOC_ADQ | VALOR_RESUMEN
```

`VALOR_RESUMEN` es el **DigestValue de la firma**. Se extrae del XML guardado con
`domain.DigestDeXML` en vez de persistirlo aparte: es un dato derivado y tenerlo
duplicado solo abre la puerta a que discrepen.

**No generamos PDF.** El navegador lo hace desde el HTML con `@media print`.
`wkhtmltopdf` (el que usa `greenter/report`) está archivado desde enero 2023 y
arrastra **CVE-2022-35583, un SSRF de CVSS 9.8**: no cabe en un servicio que
custodia claves privadas.

La plantilla (`infra/http/impresa.html`) recibe un struct. Cuando un emisor pida su
logo, se agrega el campo y se reusa la validación anti-SSRF de `webhook/sender.go`.

## Hora de Lima: no es un detalle cosmético

**Greenter renderiza el XML en `America/Lima` siempre** (`TwigBuilder.php:70`,
`TimeZonePe::DEFAULT`). Enviarle la medianoche UTC del día 6 producía
`IssueDate 2026-08-05` — **el documento legal con una fecha distinta a la nuestra.**

Y antes de eso: una venta de las 20:00 en Lima ya es del día siguiente en UTC, así
que `time.Now()` registraba el día equivocado.

`domain/tiempo.go` resuelve ambas:

- `HoyEnLima()` — la fecha de emisión es una **fecha de calendario peruana**.
- `FechaEmisionLima(f)` — fija el día al **mediodía de Lima**, sin convertir zona
  (lo que viene de una columna `date` es un día, no un instante: convertirlo sería
  el error que esto evita). El mediodía deja margen para que ninguna conversión
  mueva el día.

Se importa `_ "time/tzdata"`: la imagen del contenedor no trae la base de zonas y
`LoadLocation` caería a UTC sin avisar.

## Límite de concurrencia hacia SUNAT

**SUNAT no publica sus límites** (verificado: no hay fuente pública). Lo único
medido es que 30 envíos simultáneos del mismo RUC producen 20 × `401`.

`infra/engine/limitador.go` serializa por RUC. **Bloquea en vez de reprogramar el
job**: para cuando la petición llega ahí, `Tomar` ya marcó el comprobante como
`procesando`, y soltarlo lo dejaría atascado hasta la ventana de rescate.

`ENGINE_MAX_POR_EMISOR` (default **1**) es la perilla. Subirla solo con medición.
El semáforo es de proceso: con varias instancias de la API habría que moverlo a un
advisory lock de Postgres.

## Validación previa: todo lo comprobable, antes del contador

Un rechazo posterior a `SiguienteCorrelativo` deja un hueco que SUNAT observa.

| Regla | Dónde |
|---|---|
| Razón social 3–100 caracteres (**error 2022**) | `domain/receptor.go` |
| RUC con dígito verificador (módulo 11) | `domain.RUCValido` |
| DNI 8 dígitos, RUC 11, catálogo 06 | `domain.ValidarReceptor` |
| Factura (y nota sobre factura) exige RUC | `domain.exigeRUC` |
| Boleta > S/700 exige documento del comprador | `TopeBoletaSinDocumento` |
| Motivo de nota: **catálogo 09 crédito, catálogo 10 débito** | `domain/nota.go` |
| Certificado: PEM completo, vigente y **clave que corresponde** | `domain/certificado.go` |

⚠️ **Los catálogos 09 y 10 son distintos.** El `06` (devolución total) existe en el
de crédito y no en el de débito. Usar el equivocado es rechazo seguro.

⚠️ **Una boleta chica sin documento es la venta normal de un restaurante.** Validar
de más ahí sería peor que no validar: por debajo de S/700 el documento es opcional.

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

## Rendimiento: medido, no supuesto

```
API Go (/readyz, incluye Postgres)   0.2 ms
Engine PHP (/health)                 0.2 ms
Emisión completa contra SUNAT        0.4 s     ← ~100% es SUNAT
30 POST concurrentes                 instantáneos, 0 huecos
```

**Go no es el cuello de botella.** Por eso no se hizo perfilado, tuning de GC,
cambio de router ni `sync.Pool`: serían horas en el 0.2 ms mientras el otro 99.9 %
lo pone un tercero. Las mejoras reales fueron de **control de flujo** (el limitador
por emisor), no de velocidad.

## Deuda conocida

- **`domain.TimeoutProcesando` está duplicado en `config`** como `timeoutProcesando`
  para que config no dependa de un feature. Si uno cambia, hay que cambiar el otro.
- El barrido de huérfanos es global: con muchos tenants convendría particionarlo.
- El limitador de concurrencia es **de proceso**: con varias instancias de la API
  hay que moverlo a un advisory lock de Postgres.
- La plantilla de impresión es una sola para todos; no hay marca por emisor.
- `/metrics` no incluye métricas del runtime de Go (se escribe el formato a mano
  para no sumar `client_golang`). Si hacen falta GC y goroutines, ahí sí conviene.
- **Sin resolver, y es legal, no técnico:** confirmar con un contador o abogado
  tributarista si custodiar el certificado del cliente es "software del
  contribuyente" o ya es actividad de PSE.

## Fuera de alcance, justificado

**Guía de Remisión Electrónica (GRE).** SUNAT no la exige para delivery de comida
preparada a consumidor final; aplica a catering empresarial y traslado de insumos
entre locales. Sería una plataforma nueva (REST + OAuth2, distinta del SOAP actual)
para un caso que un restaurante típico no tiene.
