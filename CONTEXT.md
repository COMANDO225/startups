# CONTEXT — léeme primero

Handoff para continuar en otra máquina. Este archivo es la memoria de todo lo
decidido y aprendido; el resto de documentos son el detalle.

---

## Quién es Anderson y qué busca

Desarrollador peruano (Lima, distrito de Ate). Tiene **dos trabajos estables**, no
está desempleado ni apurado por dinero inmediato. Gana ~S/8k/mes y su meta es
llegar a **S/15–25k mensuales** con software propio.

**Tiempo disponible: ~36 h/semana** — jueves y viernes de tarde-noche, y sábado y
domingo completos.

⚠️ **Ese reparto importa más que el total.** Solo ~12 h caen en horario comercial:
vender B2B de oficina únicamente es viable jueves y viernes. Las 24 h del fin de
semana solo sirven si el cliente atiende sábado y domingo (restaurantes, boticas,
veterinarias), o para construir.

Está dispuesto a vender presencial al inicio; "desde casa" para él significa que
el ingreso recurrente sea remoto, **no** que nunca salga.

---

## Cómo quiere que trabajes con él

Lo pidió explícitamente y lo sostuvo toda la conversación:

- **Crudeza antes que amabilidad.** No le digas que algo va bien porque él lo
  propuso o porque ya estaba escrito. Si la idea es mala, dilo.
- **Verifica, no supongas.** Varias veces afirmé cosas sin comprobarlas y él las
  desmintió. Cuando lo haga, corrige de frente y sigue.
- **Él corrige bien.** Cazó un error de cálculo mío, sacó datos del *network tab*
  cuando un portal no dejaba copiarlos, y me discutió conclusiones con razones.
  Trátalo como par técnico.
- **Le cuesta seguir flujos abstractos** — lo dijo él. Explica con casos concretos
  y numerados, no con teoría. Pero **es persistente**, y eso vale más que la
  velocidad en este tipo de proyecto.
- **Sin comentarios de relleno en el código.** Solo donde el *por qué* no es obvio.

---

## Estado de los proyectos

| Proyecto | Estado |
|---|---|
| **`facturacion-service/`** | ✅ **Listo para producción.** Facturas, boletas por Resumen Diario, notas, anulación, webhooks, QR e impresión, alta de emisores por API, métricas. **65 tests** + integración con Postgres real |
| **`tacu-project/`** | 📋 Solo plan de negocio. **Cero código.** Producto SaaS para restaurantes |
| Residuos sólidos (EO-RS) | ❌ Descartado con datos. Ver `docs/decisiones.md` |

**Nada está desplegado.** No hay AWS, ni CI/CD, ni entorno de producción. Todo
corre en Docker Compose local.

---

## La tesis de negocio, en una página

**Producto:** Tacu — SaaS para restaurantes peruanos.

**El dolor, verificado:** Rappi cobra 25–30 % y PedidosYa 25–32 %. Hay
restaurantes que generan el 40 % de sus ventas por apps y **apenas el 5 % de su
ganancia**. El **70 % de las cevicherías ya contrató repartidores propios** para
escapar de las comisiones — están gastando plata en un parche, que es la señal de
dolor más fuerte que existe.

**El diferenciador:** OlaClick (+28 000 restaurantes en LatAm) domina el canal
digital, pero **en su propia documentación manda a sus clientes peruanos a comprar
un back office aparte para la facturación SUNAT**. PANCA y Wally dominan la
trastienda pero tienen canal digital débil. **Nadie une las dos cosas.**

**Por qué la grieta aguanta:** OlaClick opera en 20 países; hacer SUNAT bien es
trabajo local y tedioso que sirve solo a Perú. Y **SUNAT se endurece en 2026**
(GRE sancionable desde julio, códigos nuevos en agosto), lo que lo hace todavía
menos rentable para ellos. **El foso es ser más peruano de lo que a ellos les
conviene ser.**

**Pero es brecha de producto, no monopolio.** OlaClick puede integrar un PSE
cuando quiera. Se gana tiempo, no exclusividad.

**Precio:** S/119 (Operación) y S/299 (Crecimiento). Verificado que el mercado
paga eso: Wally cobra S/199+IGV y OlaClick Elite ~S/255. **No es el techo de S/119
que temíamos.**

**Meta:** ~150–180 clientes para S/25k netos. Son 3–4 años, no 18 meses.

---

## ⚠️ Lo que NO está validado (y sostiene todo)

**Anderson nunca ha hablado con un dueño de restaurante.** Cero. La tesis entera
descansa en investigación de escritorio.

**Lo primero que debe hacer, y cuesta un sábado:**

> *"¿Qué sistema usas para pedidos y cuál para facturar? ¿Son el mismo?"*
> *"El pedido que entra por WhatsApp, ¿lo vuelves a escribir para hacer la boleta?"*

**Criterio de decisión, fijado de antemano:** si ≥ 3 de 5 dueños re-digitan y lo
mencionan como molestia → construir. Si < 3 → el dolor no existe, cerrar con
evidencia y cambiar de vertical.

**No lo dejes saltarse esto.** Ya se descartó un vertical entero (residuos) por
construir sobre una premisa sin verificar.

---

## Lecciones de método — no las repitas

**1. Buscar por categoría de producto, no por nombre del sector.**
Se buscó *"software para restaurantes Perú"* → apareció PANCA con 250 clientes →
conclusión falsa de "mercado 0.4 % penetrado". La categoría real era *"pedidos
directos por WhatsApp sin comisión"*, y ahí estaba OlaClick con 28 000. **Buscar
mal es peor que no buscar: devuelve un resultado tranquilizador y falso.**

**2. Una sanción escrita en la norma no significa dolor real.**
El plan original vendía "multas millonarias" en residuos. Los datos del OEFA
mostraron **29 empresas privadas sancionadas en 14 años** (~0.2 % anual). El
discurso del miedo estaba muerto y nadie lo había comprobado.

**3. Verifica la API antes de escribirla.**
`setFechaGeneracion` no existe; es `setFecGeneracion`. Se descubrió inspeccionando
la clase dentro del contenedor, después de fallar por suponerlo.

**4. Los defaults ocultos anulan tu configuración.**
River cancela el contexto del job a los 60 s por defecto. Toda la cadena de
timeouts era decorativa mientras ese default estuviera activo.

**5. Rompe cosas a propósito.** De los 7 bugs de correctitud encontrados, ninguno
salió de tests unitarios: salieron de matar contenedores a mitad de un flujo.

**6. La prueba de carga encuentra lo que la funcional no.** El servicio pasaba
todos sus tests y emitía bien de a uno. **30 emisiones concurrentes dejaron 30 de
31 rechazadas**, por dos bugs invisibles en uso normal: un fallo de red se
clasificaba como rechazo definitivo (matando el comprobante y quemando el
correlativo), y no había ningún límite de concurrencia hacia SUNAT.

**7. Verifica la zona horaria cuando el dominio es de un país.** Greenter renderiza
el XML en `America/Lima` siempre. Enviándole medianoche UTC, **el documento legal
salía con un día menos que nuestro propio registro** — y nadie lo habría notado
hasta una fiscalización.

---

## Decisiones técnicas cerradas (no re-litigar)

| Tema | Decisión | Por qué |
|---|---|---|
| Cola | **River**, no Asynq | Encola dentro de la transacción → sin dual-write |
| HTTP | **Fiber v3.4** | Reusa `shared/` de pichangealo. `StructValidator` elimina el middleware de bind |
| Firma XML | **Nunca reimplementarla** | La canonicalización C14N debe dar bytes idénticos. Greenter la resuelve |
| Motor | **PHP + Greenter** en contenedor stateless | El certificado viaja por request → multi-tenant real |
| Repo del servicio | **Separado de Tacu** | Servirá también a boticas y otros |
| Emisor SUNAT | **El restaurante, no Anderson** | Modelo SEE-Contribuyente. Él nunca necesita su Clave SOL |

**Estructura de código:** se conserva DDD + CQRS (a él le gusta y encaja), pero sin
las 5 ceremonias que se midieron en `pichangealo-backend`: sin `infra/http/contract/`,
puertos en un solo lugar, `Reconstruir(struct)` en vez de parámetros posicionales,
un mapper por feature, y **tests desde el primer día**.

---

## Lo que sigue

**Negocio (bloqueante):** las 5 conversaciones con dueños de restaurante.

**Técnico en `facturacion-service`: nada bloqueante.** Lo que quedaba pendiente se
cerró (QR e impresión, alta de emisores, notas verificadas contra BETA, alerta de
atascados, límite de concurrencia, FrankenPHP). La deuda restante está en
`facturacion-service/CLAUDE.md` y toda es de escala, no de correctitud.

**Y falta desplegarlo.** No hay AWS, ni CI/CD, ni dominio. Todo sigue en Docker
Compose local. Con 0.4 s por emisión y ~100 % de esa latencia puesta por SUNAT,
**un VPS chico (~US$10–20/mes) con el mismo compose + Caddy alcanza y sobra.** No
hace falta AWS ni Kubernetes.

Antes del primer cliente real:

- 🔴 **Respaldar `CRYPTO_MASTER_KEY` fuera del servidor.** Si se pierde, los
  certificados cifrados de todos los clientes son irrecuperables.
- 🔴 **Backups de Postgres verificados restaurando**, no solo configurados.
- 🟠 `ADMIN_TOKEN` y `METRICS_TOKEN` con valores reales, `APP_ENV=production`.

---

## La pregunta legal: investigada, y la respuesta es buena

Se temía que custodiar el certificado del cliente obligara a registrarse como PSE.
**No es así.** SUNAT tiene tres figuras y la tercera no exige registro alguno:

| Figura | Qué hace | Qué exige |
|---|---|---|
| **OSE** | **Valida** los comprobantes *en lugar de* SUNAT | 300 UIT = **S/1,650,000** + carta fianza |
| **PSE** | Emite por cuenta del contribuyente, con **su propio** certificado | 150 UIT = **S/825,000** + 5 trabajadores + ISO 27001 |
| **SEE‑Del Contribuyente** | El contribuyente emite con **su propio** certificado, desde software propio **o de un tercero** | **Nada.** Sin registro, sin capital, sin homologación |

**El criterio que separa PSE de contribuyente es de quién es el certificado:**

> *"El PSE ofrecerá sus servicios utilizando **su propio certificado digital** para
> la firma de los comprobantes de pago electrónico, **y no el del contribuyente**."*

Nuestro servicio firma con el certificado **del restaurante** y envía con **su**
Clave SOL. El emisor ante SUNAT es el restaurante. → **SEE‑Del Contribuyente.**

Dos datos que además quitan fricción:

- **La homologación fue eliminada** (R.S. 287‑2017/SUNAT). Se emite directo contra
  producción, sin filtro previo. No hay trámite de alta técnica por cliente.
- **El contraste que lo confirma:** NubeFact **sí** es PSE y OSE, y su pitch es
  *"no necesitas certificado digital, nuestro servicio lo incluye"*. Modelo opuesto.

⚠️ **UIT 2026 = S/5,500** (DS 301‑2025‑EF). Los montos de arriba se mueven cada año.

### Los dos secretos: no son el mismo riesgo

Esta es la parte que importa operativamente, y no es la que se temía:

| Secreto | Qué permite | ¿Se puede limitar? |
|---|---|---|
| **Certificado digital** | Firmar documentos **como si fuéramos el cliente** | ❌ **No.** Poder de firma absoluto, inherente al modelo |
| **Clave SOL** | Declarar impuestos, ver su información tributaria, tocar su RUC | ✅ **Sí**, a "solo emitir comprobantes" |

**Pedir siempre un usuario secundario, nunca la Clave SOL principal.** SUNAT permite
crear usuarios secundarios con perfil restringido. El WSSE autentica igual
(`RUC + usuario`), así que **no hay que cambiar una línea de código**: es un paso
de onboarding.

Lo crea **el cliente**, no nosotros — solo el dueño de la Clave SOL principal puede.

> **Y esto es argumento de venta, no solo higiene.** Decirle a un dueño *"no me des
> tu Clave SOL, créame un usuario que solo facture"* es lo contrario de lo que pide
> cualquier proveedor. Convierte el mayor pasivo en señal de confianza.

### Lo que sigue sin resolver

**La norma no regula expresamente que un tercero custodie el certificado.** No hay
prohibición, pero *no regulado* ≠ *permitido*. Las tres preguntas para el contador:

1. Firmando con el certificado y la Clave SOL del cliente, ¿algo obliga a
   inscribirse como PSE, o basta el SEE‑Del Contribuyente?
2. ¿Hay reparo en custodiar el certificado de un tercero, aunque esté cifrado?
   ¿Qué debe decir el contrato sobre responsabilidad ante rechazo o filtración?
3. ¿Se puede exigir por contrato el **usuario secundario** y no la clave principal?

**Fuentes:** [SUNAT — PSE](https://cpe.sunat.gob.pe/aliados/pse) ·
[SUNAT — OSE](https://cpe.sunat.gob.pe/aliados/ose) ·
[SUNAT — SEE Del Contribuyente](https://cpe.sunat.gob.pe/sistema_emision/see_contribuyente) ·
[SUNAT — usuarios secundarios de la Clave SOL](https://orientacion.sunat.gob.pe/07-creacion-de-usuario-secundarios-de-la-clave-sol) ·
[Fin de la homologación (R.S. 287-2017)](https://noticierocontable.com/adios-al-proceso-homologacion/) ·
[Valor de la UIT 2026](https://lpderecho.pe/valor-uit-2026-decreto-supremo-301-2025-ef/)

---

## Mapa de documentos

| Archivo | Qué tiene |
|---|---|
| **`CONTEXT.md`** (este) | La memoria. Empieza aquí |
| `README.md` | Índice del monorepo |
| `docs/decisiones.md` | Historia: qué verticales se descartaron y con qué evidencia |
| `docs/guion-entrevistas.md` | Las preguntas de campo. Sirve para cualquier vertical |
| `tacu-project/README.md` | Plan de producto de Tacu: mercado, competencia, precios |
| `facturacion-service/CLAUDE.md` | **Memoria técnica.** Glosario SUNAT, reglas de correctitud, todos los bugs y por qué |
