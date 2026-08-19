# Tacu

SaaS para restaurantes peruanos: el dueno sube la foto de su carta, una IA la
extrae y se publica un catalogo web. El backend esta en `tacu-backend/` (Go,
modulo `tacu-backend`).

Esto solo anota lo que NO se deduce leyendo el codigo. La estructura de carpetas
se ve, y el porque de cada decision esta en los comentarios del archivo que la
implementa. Aqui van las decisiones que se perderian si nadie las escribe.

## Como se escribe aqui

- Todo en espanol: nombres, comentarios, mensajes de error. Sin tildes ni enes
  con virgulilla en el codigo. Sin emojis.
- Los comentarios explican POR QUE, no QUE. Se documenta la decision y la trampa
  que se encontro, no lo que la linea ya dice.
- Cero dependencias nuevas sin avisar. No se editan `go.mod` ni `go.sum` por
  iniciativa propia: si hace falta una, se pide primero.

## Comandos

`make ayuda` lista todo (en `tacu-backend/`). Si un comando hay que teclearlo de
memoria, es que falta un objetivo en el Makefile.

- Postgres esta en el **5433**, no en el 5432. Es a proposito, para no chocar
  con el Postgres de otro proyecto y perder media hora averiguando por que los
  tests hablan con la base equivocada.
- El DSN entra por `TACU_BD_DSN`. Es una credencial: no va en `config/`.
- Las claves de IA salen de `.env` (`GEMINI_API_KEY`, `OPENAI_API_KEY`).

## Dinero

- El dinero es `int64`. Nunca `float64` cerca de un monto: `0.1 + 0.2 != 0.3` y
  el error se acumula en silencio hasta que un total no cuadra.
- `dinero.Centimos` para soles, `dinero.MicrosUSD` para dolares. Son tipos
  DISTINTOS a proposito: los dos son `int64` y los dos son dinero, asi que sin
  la separacion nada impide sumar el precio de un ceviche con lo que costo una
  llamada a Gemini, y el compilador no diria nada.
- Micros y no centimos para el dolar porque una foto cuesta $0.0336, que en
  centimos de dolar es un decimal, o sea justo lo que no queremos.

## El precio de un plato

El precio NUNCA es solo el numero que dijo el modelo. Vienen dos testigos —el
texto impreso tal cual y la interpretacion numerica— y Go los cruza parseando el
texto por su cuenta. Si no cuadran, el plato se marca para revision y no se
publica asi.

Existe porque los modelos de vision CORRIGEN en silencio lo que perciben como
errores de formato: la salida es un JSON perfecto con el precio equivocado, y el
JSON Schema valida la forma, no los valores. Sin el cruce no hay ninguna senal.

Detalle en `internal/modules/carta/domain/carta.go` (`MotivoRevision`,
`verificarPrecio`).

## Modelos de IA

- Se eligieron MIDIENDO, no leyendo benchmarks. Antes de cambiar uno hay que
  correr `cmd/cartabench` (extraccion) o `cmd/fotocheck` (fotos) y comparar
  contra lo que ya esta.
- Caso documentado: en Roboflow Vision Evals `3.5-flash` sale #1 global (86.6%)
  y `3.7-flash` #2 (84.6%). Sobre la carta real, el de 84.6% gano 3-0. Los
  benchmarks genericos no miden esta tarea, y el eje que decide no es la
  exactitud media sino la varianza: que tan mal falla cuando falla.
- Cambiar de modelo, de proveedor o de orden se hace en `config/config.yaml`,
  cero codigo. Las mediciones y el criterio de cada eleccion estan ahi mismo.
- La aplicacion pide TAREAS (`ai.LeerCarta`, `ai.GenerarFoto`), nunca un modelo.

## Generacion de imagenes

Nunca se le pide al generador una fraccion ("1/4 de pollo"): se traduce antes a
piezas anatomicas (una pierna, un muslo) mas una escala relativa a algo visible
en el plato. Un generador no divide un pollo; reconoce objetos, y ante una
fraccion revierte al ave entera, que es lo estadisticamente probable.

Los descriptores fotograficos van en INGLES a proposito, aunque el resto del
proyecto sea espanol. Ver `internal/modules/carta/domain/porcion.go`.

## Archivos e imagenes

- Las imagenes se guardan por CLAVE, nunca por URL. La URL la arma quien sirve.
  Asi mudarse de disco a R2 no reescribe lo ya guardado.
- Toda ruta que venga de una URL se abre con `os.Root`, nunca concatenando con
  la raiz. El confinamiento lo hace el sistema operativo, no la memoria de quien
  llama.

## Tests

- En verde con `-race`. Es la condicion, no un extra.
- Los tests que GASTAN DINERO van detras de una variable de entorno y hacen
  `t.Skip` sin ella: `TACU_E2E=1`. Nunca se quitan las guardas.
- `cmd/cartacheck`, `cmd/cartabench` y `cmd/fotocheck` son CLIs de laboratorio y
  llaman a la IA de verdad: cada corrida cuesta plata. No se lanzan "para ver".

## Base de datos

- Migraciones con goose en `sql/migrations/`, y sqlc genera
  `internal/modules/carta/adapters/postgres/cartadb/`. Ese paquete es generado:
  se cambia la query o el `sqlc.yaml` y se corre `make generar`, nunca se edita
  a mano.
