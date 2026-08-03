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
| **`facturacion-service/`** | ✅ **Funciona contra SUNAT BETA.** Facturas, boletas por Resumen Diario, anulación, webhooks. 21 tests |
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

**Técnico en `facturacion-service`:**
1. Representación impresa con **QR** — requisito legal, sin construir
2. Alta real de tenants (hoy solo existe `cmd/seed`)
3. Que algo **avise** cuando `/v1/comprobantes/atencion` deja de estar vacío
4. Probar notas de crédito/débito contra BETA (el motor las construye, no se ejercitó)

**Antes de producción:** confirmar con un contador o abogado tributarista si un
SaaS que **custodia el certificado del cliente** es "software del contribuyente" o
ya es actividad de PSE. Es barato preguntarlo y caro descubrirlo con 80 clientes.

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
