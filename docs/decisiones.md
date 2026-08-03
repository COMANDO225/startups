# Historia de decisiones

Qué se evaluó, qué se descartó y con qué evidencia. Sirve para no volver a
recorrer el mismo camino.

> **[VERIFICADO]** = fuente oficial o pública. **[ESTIMADO]** = hipótesis sin confirmar.

---

## Verticales evaluados y descartados

| Vertical | Por qué se descartó |
|---|---|
| **Residuos sólidos (EO-RS)** | El dolor regulatorio no existe (ver abajo) |
| **Boticas** | 8+ competidores locales, y varios venden **licencia perpetua**: el sector está entrenado contra la suscripción |
| **Dental** | 8+ competidores. El Colegio Odontológico denuncia **sobrepoblación de odontólogos**: su dolor es *conseguir* pacientes, no administrarlos. Eso es un marketplace, no un SaaS |
| **Gimnasios** | Gym Control cobra **S/750 al año** (S/62/mes). Ancla de precio imposible, y el gimnasio promedio tiene 2–5 personas |
| **Estética** | *"La mayor parte de la actividad ocurre fuera del control de las autoridades sanitarias y tributarias"*. **Un negocio informal no compra software que genera trazabilidad fiscal** |
| **POS textil** | Competencia brutal, precio anclado por la commoditización de la facturación electrónica |
| **Veterinarias** | 🟡 Único sobreviviente además de restaurantes. 4 950 locales, mercado creciendo fuerte, 4–6 competidores. **Plan B si restaurantes falla** |

---

## Residuos sólidos: cómo se descartó

Era el proyecto original. Tenía cero competencia de software y ARPU alto
(S/1 000/mes, solo 20 clientes para la meta). Se cayó por datos.

**El plan original decía** que las EO-RS pagarían por evitar "multas millonarias
del OEFA".

**[VERIFICADO] La realidad**, sacando el dataset completo del RUIAS (el portal
público del OEFA) — 361 entidades sancionadas en el histórico del subsector:

| | Entidades | Infracciones | UIT | Soles |
|---|---|---|---|---|
| Municipalidades y Estado | **332** | 747 (85 %) | 9 689 | S/53.3 M |
| **Empresas privadas** | **29** | 136 (15 %) | 2 166 | S/11.9 M |

```
Solo 29 empresas privadas sancionadas en ~14 años, contra ~1 067 EO-RS
  probabilidad anual  ≈ 0.2 %
  costo esperado      ≈ S/810/año
  costo del software  = S/12 000/año
```

**El motor del miedo no existía.** Y las 29 que cayeron eran operadores de
infraestructura sancionados por **manejar mal los residuos físicamente**, no
transportistas multados por papeleo.

Quedaba un solo motor sin validar (*"sin manifiesto no hay cobro"*) que venía de
la misma fuente que ya había fallado dos veces. Sumado a que las EO-RS solo
atienden en horario de oficina —inutilizando 24 de las 36 horas disponibles—, se
cerró.

**Lo que sí dejó:** el listado público de EO-RS del MINAM y el RUIAS del OEFA son
listas de prospectos gratuitas y precalificadas. La técnica sirve para cualquier
vertical regulado.

---

## Por qué restaurantes

**[VERIFICADO]** 60 000 restaurantes formales en Perú, 25 000 en Lima.

**El dolor está probado por comportamiento, no por encuesta:**

- Rappi cobra **25–30 %**; PedidosYa **25–32 %**
- Hay restaurantes que generan el **40 % de sus ventas por apps y apenas el 5 % de su ganancia**
- El **70 % de las cevicherías ya contrató repartidores propios** para escapar

> Nadie tiene que convencerlos de que duele: ya están gastando plata en un parche.

**Y encajan con el horario:** abren sábado y domingo, que es cuando Anderson tiene
24 de sus 36 horas.

### El competidor que casi mata la tesis

**OlaClick**: +28 000 restaurantes en LatAm, 20 países, +130 000 pedidos solo en
Perú. Plan gratis permanente, pedidos por WhatsApp, asistente de IA, sin comisión.
**Todo lo que se había diseñado como diferencial, ellos ya lo tenían.**

Apareció **buscando un nombre de dominio**, después de seis mensajes diseñando
producto.

### La grieta que lo salva, en palabras del propio OlaClick

De su guía pública comparando alternativas en Perú:

> *"Cuando el volumen de tu salón exija **facturación electrónica intensiva
> (boletas y facturas SUNAT)** será el momento ideal para **complementar tu
> operación sumando un software de back office tradicional** como Restaurant.pe
> o Flexipos."*

**Le dicen a sus clientes peruanos que compren un segundo sistema.**

```
FRENTE (canal digital)          ATRÁS (operación peruana)
OlaClick: fuerte                PANCA / Wally / Restaurant.pe: fuerte
· WhatsApp + IA                 · Facturación SUNAT
· Carta QR, sin comisión        · Inventario, KDS, mesas
· Plan gratis                   · Canal digital débil
✗ SIN facturación SUNAT         ✗ Sin agente IA / marketing
```

**Nadie cruza el puente.** El restaurante que quiere ambas cosas paga dos
mensualidades, dos soportes, y data que no se cruza.

### Los ~23 competidores

OlaClick · PANCA · Wally (respaldo Culqi/Credicorp) · Restaurant.pe · Flexipos ·
Ordena Fácil · RestoBar.pe · Byte POS · Toteat (CL) · Soft Restaurant (MX) ·
Niubiz · Culqi · PideRápido · Sizi · Riqra · Smart POS · Bsale · Tecnotrends ·
Smart System · Systutor · Pebetspro · Sistema Resto Perú

⚠️ **Solo se verificaron a fondo OlaClick, PANCA y Wally.** No se comprobó si
Ordena Fácil, RestoBar.pe, Byte POS o Flexipos ya unen frente y trastienda. **Es
el riesgo abierto más grande de la tesis.**

---

## El techo de precio: corregido

Se temía que S/119 (PANCA) fuera el máximo pagable en Perú. **Falso [VERIFICADO]:**

- **Wally: S/199 + IGV = S/235/mes**, con respaldo de Culqi/Credicorp
- **OlaClick Elite: ~S/255** · Infinity: ~S/635

**S/235–255 es un precio que el mercado peruano ya paga.**

### Y una lección sobre por qué PANCA cobra poco

| | Titular | Precio |
|---|---|---|
| **PANCA** | *"El software para restaurantes más completo de Perú"* | S/99–119 |
| **Owner.com** (USA) | *"The AI platform restaurants use to grow"* | **USD 499** |

**PANCA vende ORDEN. Owner vende CRECIMIENTO.** Buena parte de la diferencia no es
el país: es el posicionamiento. Un producto peruano vendido como *"te hace vender
más"* no cobra USD 499, pero tampoco tiene por qué cobrar S/119.

---

## Criterios de selección de vertical

Derivados de las restricciones reales de Anderson. Sirven para evaluar cualquier
idea futura:

| # | Criterio | Por qué |
|---|---|---|
| 1 | **¿Abre fin de semana?** | Triplica sus horas vendibles |
| 2 | **¿Se entra sin filtro?** | Sin recepcionista ni gerente = 5× más conversaciones |
| 3 | **¿Hay gremio o colegio?** | Canal institucional en vez de puerta a puerta |
| 4 | **¿Paga suscripción?** | Si el sector usa licencia perpetua, no hay SaaS |
| 5 | **¿Menos de 3–4 competidores locales?** | |
| 6 | **¿Gatillo reciente?** | Algo cambió que lo hace necesario hoy |
| 7 | **ARPU × clientes alcanzables** | ¿Llega a la meta con sus horas? |

### El patrón incómodo, y es normal

> Donde el cliente es fácil de alcanzar, ya hay 8 competidores.
> Donde no hay competencia, el dolor no está confirmado.

Si un mercado es accesible y el dolor es obvio, ya llegaron ocho antes. La única
forma de encontrar un mercado sin competencia es que sea difícil de alcanzar o que
el dolor no sea evidente — y en ambos casos hay que **validar antes de construir**.

---

## Sobre el nombre "Tacu"

De *tacu tacu*. Verificado sin colisiones y con `tacu.pe` libre (agosto 2026).

**Cómo funciona una marca en Perú [VERIFICADO]:** la protección es **por clase de
Niza**. Dos marcas idénticas coexisten en clases distintas; INDECOPI solo deniega
si hay riesgo de confusión dentro de la misma clase. La del software es la **42**;
los restaurantes se registran en la **43**, así que un restaurante homónimo no
bloquea.

Murieron por verificación: **Chaski** (Chazki, casi-unicornio de logística),
**Rimay** (5 empresas, una de software), **Wayku**, **Kanka**, **Uchu**, **Tulpa**
(ERP dormido), **Miski** (diluidísimo en el rubro alimentos).

**Lección:** toda palabra quechua que significa *rico/comida/comer* ya la usan
decenas de restaurantes. **El producto vende *a* restaurantes, no es uno** — no
conviene nadar en su piscina de nombres.

---

## Fuentes

**Restaurantes**
- [Comisiones de apps de delivery en Perú 2026](https://www.panca.pe/blog/comisiones-apps-delivery-peru-comparativa)
- [70 % de cevicherías contratan repartidores propios](https://www.peru-retail.com/el-70-de-cevicherias-contratan-repartidores-propios-por-altas-comisiones-en-apps-de-delivery/)
- [**OlaClick admite que hay que sumar un back office para SUNAT**](https://olaclick.com/es/ponto-de-venda/las-mejores-alternativas-para-restaurantes-en-peru/)
- [PANCA](https://www.panca.pe) · [Wally precios](https://soluciones.miwally.com/precios/) · [Owner.com](https://www.owner.com/)

**Residuos (descartado)**
- [RUIAS — Administrados Sancionados, OEFA](https://publico.oefa.gob.pe/administrados-sancionados/#/)
- [Listado de EO-RS autorizadas — MINAM](https://www.gob.pe/institucion/minam/informes-publicaciones/274465-listado-de-empresas-operadoras-de-residuos-solidos-autorizadas-por-el-minam)

**Otros verticales**
- [Establecimientos farmacéuticos — DIGEMID](https://www.digemid.minsa.gob.pe/Archivos/Boletines/Establecimientos/EEFF-06-23.pdf)
- [Sobrepoblación de odontólogos — Colegio Odontológico](https://ccdp.org.pe/noticias/sobrepoblacion-de-odontologos-en-el-peru-tiene-como-una-de-sus-consecuencias-a-la-publicidad-enganosa-afirmo-el-decano-nacional-del-cop)
- [Mercado veterinario en Perú](https://elcomercio.pe/economia/dia-1/del-veterinario-de-barrio-a-las-clinicas-especializadas-asi-crece-el-mercado-de-salud-para-mascotas-en-el-peru-cadenas-veterinarias-noticia/)
- [Industria estética fuera del control tributario](https://www.infobae.com/peru/2025/11/08/cirugias-plasticas-esteticas-en-peru-una-industria-que-mueve-millones-cuanto-cuesta-un-tratamiento-y-que-operaciones-son-las-mas-demandadas-maria-del-carmen-martinez/)

**Marca**
- [Clasificación de Niza en INDECOPI](https://www.brandia.pe/blog/clasificacion-niza-indecopi-peru)
