# Redibujar el logo del letrero: low contra medium

**Estado: decidido — medium 1024x1024.** Medido el 10-sep-2026 con
`cmd/logocheck` sobre el letrero de Galponcito, una pizarra plastificada
fotografiada con el telefono: reflejos, arrugas del plastico y torcida.

Las imagenes NO se versionan. Se regeneran con el comando de abajo; lo que se
guarda son los numeros y lo aprendido.

```
go run ./cmd/logocheck -foto /ruta/al/letrero.jpg -calidades low,medium -tamano 1024x1024
```

## Lo medido

| calidad | $/imagen | segundos | resultado |
|---|---|---|---|
| low    | 0.006 | ~15 | texto limpio, MASCOTA DEFORMADA: el pico se le vuelve de pato y el sombrero se funde con el cuerpo |
| medium | 0.053 | ~38 | reconstruye el pollo con su sombrero, su panuelo y sus guantes |

Se paga el 9x. La sorpresa fue **donde** se cae `low`: no en el texto, que es lo
que se temia, sino en el DIBUJO — y el dibujo es la parte que hace reconocible un
logo. El texto sale bien en las dos, con la enie incluida.

`high` (1536x1024, $0.165) no se probo: el logo se pinta en la cabecera de un
catalogo en un telefono, y son 3x el precio por pixeles que nadie ve.

## El bug que encontro esta medicion, y que valia mas que la medicion

La PRIMERA corrida devolvio **"LOS POLLOS HERMANOS"** con low y **"TORCHY'S
TACOS"** —una cadena real de EE.UU., con su simbolo (R)— con medium. Ninguna se
parecia a Galponcito.

No era el modelo. El proveedor de OpenAI construia el cuerpo asi:

    cuerpo := map[string]any{"model": modelo, "prompt": pet.Prompt, "n": 1}

`pet.Referencias` no se leia NUNCA, y posteaba a `/images/generations`. La foto
del letrero no salia de nuestro servidor: el modelo recibia "redibuja el logo de
esta fotografia" sin fotografia, y rellenaba el hueco con un logo famoso que
recordaba.

El hueco era invisible porque las fotos de plato van por Gemini, que si manda las
referencias; OpenAI solo estaba configurado para `generar_logo`, que no se habia
llamado nunca.

Se arreglo con `/images/edits` y multipart. Un detalle que costo otra corrida:
`CreateFormFile` pone `application/octet-stream` y OpenAI lo rechaza con
`unsupported_file_mimetype` — hay que escribir el Content-Type de cada parte a
mano.

## Lo que esto deja escrito para el producto

Ninguna de las dos sale IDENTICA al original: esto reinterpreta, no restaura. Por
eso el redibujo se ofrece como propuesta al lado del letrero y lo acepta el
dueno, en vez de sustituirle la marca en silencio.

Coste de esta tanda: $0.21, contando la corrida que tiro el bug.
