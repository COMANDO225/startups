# Generar fotos de platos: OpenAI contra Gemini

**Estado: sin decidir.** Falta confirmar el precio real de Gemini y que el juicio
visual lo haga una persona.

Las imagenes NO se versionan (11 MB de blobs). Se regeneran con los comandos de
abajo; lo que se guarda son los numeros.

## Como se genero

Mismo prompt para los dos modelos —afinarle el prompt a cada uno mide mi
habilidad escribiendo prompts, no los modelos— y el mismo tamano final,
1024x1024:

```
go run ./cmd/fotocheck -modelos openai/gpt-image-2 -repeticiones 2 \
   -salida experimentos/fotos-generadas -tamano 1024x1024 -calidad medium "Ceviche Mixto"

go run ./cmd/fotocheck -modelos gemini/gemini-3.1-flash-image -repeticiones 2 \
   -salida experimentos/fotos-generadas -tamano 1K -proporcion 1:1 "Ceviche Mixto"

go run ./cmd/fotocheck -modelos gemini/gemini-3.1-flash-lite-image -repeticiones 2 \
   -salida experimentos/fotos-generadas -proporcion 1:1 "Ceviche Mixto"
```

Dos platos: "Ceviche Mixto" (de la carta de La Tribuna del Sur) y "1/4 de pollo a
la brasa con papas fritas y ensalada" (de Galponcito, ya expandido: la carta dice
solo "1/4 pollo").

## Lo medido — 14-ago-2026

Tiempo cronometrado por el CLI, 4 imagenes por modelo. Precios verificados contra
la documentacion oficial de cada proveedor el mismo dia.

```
modelo                              tiempo (4 corridas)   media   $/img   x60    peso
gemini/3.1-flash-lite-image        3.7  3.9  4.4  4.8 s   4.2 s  $0.0336  $2.02  ~780 KB
gemini/3.1-flash-image             9.2  9.4  9.6 10.0 s   9.6 s  $0.0670  $4.02  ~880 KB
openai/gpt-image-2 medium         46.5 46.9 49.3 52.6 s  48.8 s  $0.0530  $3.18  ~1.9 MB
```

**Lite gana en las tres dimensiones a la vez: 12x mas rapido que OpenAI, el mas
barato, y el unico que entra en el presupuesto de $3.00 por importacion.**

OpenAI ademas es erratico: en una corrida suelta anterior tardo 16.7 s y en estas
cuatro no bajo de 46 s. Esa varianza importa para la pantalla de "generar todas":
no se le puede decir al dueno cuanto falta.

Carta de 60 platos con 4 llamadas en paralelo: **Lite ~1 min, flash-image ~2.5 min,
OpenAI ~12 min.**

Ninguno de los dos proveedores publica cifras de latencia. Estos segundos son
medidos aca y no existen en ninguna documentacion.

## Los precios, y en cual me habia equivocado

El `$0.067` de flash-image que tenia cargado a ojo **resulto correcto**. El que
estaba mal era el de OpenAI: tenia `$0.041`, que es el precio de medium
**1536x1024**, y yo estaba midiendo a **1024x1024**, donde medium cuesta `$0.053`.
Subestimaba la carta entera en $0.72 y la daba por dentro del presupuesto cuando
se pasa.

De ahi sale una limitacion que quedo documentada en `config.yaml`: un solo
`por_imagen` no puede representar a gpt-image-2, que va de `$0.006` en low a
`$0.165` en high 1536x1024 — un rango de 27x. Mientras el producto use una sola
combinacion el estimado sirve; el dia que se ofrezcan dos calidades, la tabla
tiene que dejar de ser plana.

## Dos cosas que se aprendieron por el camino

**Los tamanos no se traducen entre proveedores.** OpenAI pide `1024x1024` y una
calidad; Gemini pide `1K`/`2K`/`4K` y una proporcion aparte. Pasarle `1024x1024`
a Gemini devuelve `Unsupported image_size`. Por eso `ai.Peticion` lleva `Tamano`,
`Proporcion` y `Calidad` por separado y no los traduce: inventar equivalencias que
ninguno de los dos garantiza es peor que no traducir.

**Gemini sin proporcion explicita devuelve 16:9.** El primer intento salio en
1408x768 contra los 1024x1024 de OpenAI, lo que hacia injusta la comparacion
visual. Hay que fijar `-proporcion 1:1`.

## Fechas de apagado que hay que vigilar

Ninguna nos afecta hoy —lo verifique— pero conviene tenerlas escritas:

```
2026-08-17  imagen-4.0-generate-001 / -fast / -ultra   (en 3 dias)
2026-10-02  gemini-2.5-flash-image  (el Nano Banana original)
2026-10-23  gpt-image-1
2026-12-01  gpt-image-1.5, gpt-image-1-mini
```

Ojo con un error de la propia documentacion de Google: la pagina de deprecations
da como reemplazo `gemini-3.1-flash-image-preview`, un ID que ya no existe. Va sin
el sufijo `-preview`.

Y un detalle que rompe en produccion: si se pasa `image_size`, la K va en
MAYUSCULA. `"1k"` lo rechaza la API.

## Lo que falta

1. **Que una persona mire las 12 imagenes y diga cual se ve mejor.** Eso no lo
   decide ninguna metrica, y es lo unico que falta para cerrar la eleccion.
2. Probar `512` y ver si alcanza para una tarjeta de catalogo.
3. Anclar con una foto real del plato (`Peticion.Referencias`): los dos
   proveedores lo aceptan y esta implementado, pero no esta medido. Hay evidencia
   de que la identidad fina del producto deriva al anclar, asi que hay que asumir
   que deriva y medir cuanto.
4. Lo que la investigacion NO pudo confirmar: no existe ni un paper ni un
   benchmark cultural que cubra comida peruana. Ningun numero sobre "que tan bien
   sale un ceviche" es citable — hay que medirlo aca. Lo que si esta medido y
   aplica: los nombres compuestos tienden a descomponerse en ingredientes sueltos,
   y describir el plato terminado puntua mejor que dar solo el nombre.
