# Detectar stickers amarillos con OpenCV

**Estado: descartado.** Se guarda porque las mediciones sirven, no el codigo.

## Que se buscaba

Los restaurantes corrigen precios pegando stickers amarillos encima del precio
impreso. Publicar el precio de abajo le hace perder plata al dueno. La idea era
detectar esos stickers con vision clasica —barato, instantaneo, deterministico—
y darle al modelo las coordenadas de donde mirar.

## Por que se descarto

**El VLM ya los detecta solo: 19 de 19, en 5 corridas de 5.** Ver la tabla en
`config/config.yaml` y `cmd/cartabench`.

El detector encuentra DONDE hay amarillo, nunca QUE numero dice. Igual hay que
preguntarle al modelo. Es un paso que no produce nada que el modelo no produzca
ya, y cada paso tiene su propio modo de falla.

## Los intentos

| | que probo | resultado |
|---|---|---|
| `1-hsv/` | umbral HSV crudo sobre el canal amarillo | detecta, pero con ruido |
| `2-morfologia/` | cierre + apertura, filtro por area / llenado / proporcion | 12 blobs limpios, 0 basura |
| `3-metricas/` | robustez, falsos positivos, y deteccion del pliegue | ver abajo |

Cada carpeta tiene su `salida.txt` con los numeros de la corrida.

## Lo que se midio y vale la pena recordar

**Es rapidisimo y no depende del umbral.** 3.5 ms a 1600x1200, 10 ms en un solo
hilo (VPS barato). Y el barrido completo de umbrales —5 valores de saturacion x
5 de brillo, 25 combinaciones— dio **12 detecciones en las 25**. No hay que
calibrar nada. Tampoco lo mueven gamma 0.6-1.7, brillo +-60, ni JPEG de calidad
30. Solo cae con la foto muy torcida (+8 grados: 10 de 12).

**Cero falsos positivos en una carta sin stickers** (`prueba.jpg`: 0
detecciones).

**Pero el amarillo no distingue un sticker de un cartel.** Medido:

```
sticker "49.0"        H=35 S=215 V=238
panel "COMBO FAMILIAR" H=33 S=213 V=238   <-- identico
```

El panel amarillo del COMBO FAMILIAR tiene el mismo tono que un sticker. Solo lo
salva la geometria (es mucho mas grande), y eso es suerte de esta carta, no una
regla. En una carta con recuadros amarillos del tamano de un sticker, esto marca
publicidad como precio corregido.

**No cuenta stickers, cuenta manchas.** Son 19 precios manuscritos y detecta 12
blobs: la columna de "PARA LLEVAR" tiene 6 stickers pegados y salen como una
sola mancha de 81x210. Asi que ni siquiera sirve como testigo independiente del
tipo "OpenCV vio 19, el modelo reporto 19".

**El pliegue de una carta doblada si se puede encontrar** (`3-metricas/s5,s6`).
Buscando la columna mas oscura de la imagen: x=815 contra x~805 real, 10 px de
error. La otra idea —buscar el valle de blancura entre los dos paneles— fallo:
solo encontro los bordes de la foto. Esto queda apuntado por si algun dia hace
falta partir una carta doblada en dos paginas.

## Si algun dia se retoma

El caso que lo justificaria es una carta donde el modelo SI se equivoque con los
manuscritos. Hoy no existe ese caso. Antes de escribir una linea, correr
`cmd/cartabench` sobre esa carta y confirmar que falla.
