# Porciones de pollo a la brasa — version 5

Estado de las fotos despues de cinco rondas de correccion del dueno del producto.
Ninguna correccion fue del modelo: todas fueron del prompt.

## Lo que cada version arreglo

**v1** — el prompt pedia "1/4 de pollo a la brasa con papas fritas y ensalada".
Salio algo del tamano de medio pollo, con la ensalada encima del plato y aspecto
de pollo rostizado palido. Tres errores en una frase.

**v2** — la fraccion se dejo de mandar al modelo. Se traduce antes a piezas
anatomicas mas un ancla de escala (`domain/porcion.go`), y el reparto de
recipientes se declara en parrafos separados con verbo propio (`app/foto.go`).
Las cuatro porciones salieron diferenciadas y la ensalada se fue a su bol.

**v3** — mas brillo y jugosidad, y las cremas que de verdad acompanan: ketchup,
mayonesa, mostaza y aji verde. Ademas se corrigio un error mio de origen: yo
tenia el pollo entero como "abierto y aplanado" y en la polleria sale INTACTO,
con su forma de ave; se parte solo si el cliente lo pide.

**v4** — el brillo se habia pasado y la pechuga salia casi quemada. Se moderaron
las manchas oscuras: color parejo, unas pocas tostaduras en las puntas, y se
prohibio explicitamente el negro.

**v5** — el pollo entero no lleva las papas alrededor. Un pollo entero equivale a
cuatro cuartos, o sea un plato entero de papas: el ave va sola en su plato y las
papas en otro. Y el encuadre se cerro sobre el plato principal.

## Como quedo el reparto

```
1/8, 1/4, 1/2   pollo y papas comparten UN plato · ensalada en su bol · 4 cremas
1 entero        pollo solo en su plato · plato aparte lleno de papas · ensalada · 4 cremas
```

Eso ultimo esta en `Porcion.Reparto`, que sobreescribe el reparto normal. Vacio
significa "como se sirve siempre"; solo el pollo entero lo usa.

## Numeros

`gemini-3.1-flash-lite-image`, proporcion 1:1, una corrida por porcion:
4.1 a 4.5 segundos, $0.0336 cada una.

## Lo que sigue sin estar medido

**No existe ningun benchmark que mida porciones en generacion de imagen.** Todo
el campo mide conteo de objetos enteros. Que describir piezas anatomicas funcione
mejor que la fraccion es una hipotesis fundada, no un hecho medido, y **una
corrida por porcion no es un dato**: en la extraccion de cartas ya paso que un
modelo parecia perfecto hasta la tercera corrida.

Lo que haria falta para afirmarlo: varias corridas por porcion, contadas a mano,
y alguien del rubro diciendo si se parece.

## Regenerar

```
for p in "1 pollo" "1/2 pollo" "1/4 pollo" "1/8 pollo"; do
  go run ./cmd/fotocheck -modelos gemini/gemini-3.1-flash-lite-image \
     -salida experimentos/fotos-generadas/porciones-v5 -proporcion 1:1 "$p"
done
```

Cada imagen deja su `.prompt.txt` al lado con el texto exacto enviado.
