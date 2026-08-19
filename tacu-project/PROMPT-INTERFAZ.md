Diseña la interfaz de la pantalla de trabajo de Tacu.

## Quién la usa y dónde

El dueño de un restaurante pequeño en Perú: una pollería de barrio, una
cevichería, un chifa. No es una persona técnica. Trabaja desde su teléfono, con
datos móviles, y muchas veces con el local abierto. Su carta impresa es su
negocio: un precio publicado mal le cuesta plata o le cuesta un cliente.

No hay cuenta ni contraseña. El acceso a su carta vive en el navegador donde la
subió; desde otro equipo no se abre.

## El trabajo que tiene que hacer

Llegar de una carta de papel a una dirección web que pueda repartir por WhatsApp,
con sus platos, sus precios y una foto de cada uno.

Una IA lee la carta y genera las fotos. La IA se equivoca, y el dueño es el único
que puede saber si se equivocó: él tiene el papel delante.

## Los cuatro pasos

El dueño recorre cuatro pasos. Puede volver a cualquiera que ya haya alcanzado.
Dentro de cada paso hay etapas que ejecuta el sistema: el dueño no navega a
ellas, pero sí las espera y a veces las ve a medias.

### Paso 1 — La carta

Qué hace: escribe el nombre de su restaurante y entrega su menú, entre una y
cuatro fotos, o un PDF. Después espera.

Puede volver aquí para añadir una página que olvidó, reemplazar una foto movida
o pedir que se vuelva a leer.

Etapas del sistema:
  1. valida los archivos antes de subirlos
  2. una IA de visión lee la carta y saca platos, precios y categorías
  3. cada precio se cruza contra un segundo testigo y se marca lo que no cuadra
  4. se deduce de qué tipo es el negocio a partir de los nombres de los platos
  5. se reordenan las categorías pensando en vender
  6. se guarda

Dura unos diez segundos. Durante ese rato todavía no hay nada que revisar.

### Paso 2 — El catálogo

Qué hace: compara lo extraído contra su carta de papel y lo administra. Corrige
lo que la IA leyó mal, añade lo que se saltó, quita lo que no existe, y reordena.

El sistema le señala dónde mirar. Hay DOS niveles de señal y no significan lo
mismo:

  - BLOQUEA: algo no cuadra y no se puede publicar así. Cinco causas: el precio
    impreso no se pudo leer; el precio leído no coincide con el texto impreso;
    el plato no tiene precio; el plato no tiene nombre; el plato tiene varios
    precios y la carta no dice de qué es cada uno.
  - SOLO MIRARLO: no hay nada roto, pero conviene que lo confirme. Una causa: el
    precio está corregido a mano sobre el impreso.

Los dos niveles NO se pueden mezclar. En una carta real de 42 platos, 19 salieron
señalados y los 19 eran del segundo tipo. Si se pintan igual que los rotos, el
dueño aprende a aprobar sin leer — que es exactamente cuando se cuela el precio
que sí estaba mal.

De las cinco causas que bloquean, solo una se arregla escribiendo (ponerle nombre
a los precios de un plato que tiene varios). Las otras cuatro se arreglan mirando
el papel y corrigiendo el dato.

Etapas del sistema: al editar un plato se vuelve a verificar ese plato y se
recuenta cuántos quedan señalados en toda la carta.

### Paso 3 — Las fotos

Qué hace: decide cómo se ven sus fotos y consigue que cada plato tenga una.

Tiene cuatro maneras de conseguir la foto de un plato:
  - generarla con IA
  - subir una suya (esta nunca se pisa con una generada)
  - corregir la que ya hay describiendo qué cambiar: se le manda la foto actual
    y solo el cambio, así que sale la misma foto con esa cosa arreglada
  - quitarla

Y puede definir el ESTILO, que se aplica a las que se generen después:
  - en qué plato sirve (por defecto uno blanco redondo)
  - sobre qué fondo (por defecto blanco limpio)
  - hasta dos fotos de ejemplo suyas, de las que la IA se guía

El estilo es JERÁRQUICO: hay uno general del restaurante y cada categoría puede
sobrescribirlo campo por campo. Una categoría puede cambiar solo el fondo y
heredar el plato.

Cambiar el estilo NO rehace las fotos ya generadas. Se aplica a las siguientes.

El estilo NO es un paso: no avanza nada, tiene valores por defecto válidos y se
abre cuando el dueño quiera.

Etapas del sistema, por cada foto: se reclama el plato para que dos procesos no
paguen por la misma foto; se reserva el presupuesto ANTES de llamar; se llama a
la IA; se guarda la imagen; se marca el plato.

### Paso 4 — Publicar

Qué hace: revisa cómo lo verá su cliente y obtiene su dirección web.

No se puede publicar mientras quede un plato del nivel que bloquea. Los del nivel
"solo mirarlo" no impiden publicar.

Etapas del sistema: se arma la dirección a partir del nombre del negocio,
desambiguándola si otro restaurante ya la tiene; se mueve el puntero de
publicación; se marca la importación.

Publicar no copia nada: mueve un puntero. Por eso volver a publicar una versión
corregida es instantáneo y volver atrás es posible.

## Todo lo que el dueño tiene que poder administrar

  restaurante   nombre, tipo de negocio
  categoria     nombre, orden entre categorias
  plato         nombre, descripcion, a que categoria pertenece, orden,
                anadir uno nuevo, quitar uno que no existe
  precio        el monto, como se llama esa opcion, anadir otro, quitar
  foto          generar, subir la suya, corregir la actual, quitar
  estilo        plato base, fondo, fotos de ejemplo — general y por categoria
  la carta      anadir mas paginas, volver a leerla

Un plato puede tener VARIOS PRECIOS: el mismo plato en tamaño personal y
familiar, escrito en la carta en una sola fila con dos montos. No son dos platos:
es un plato con dos formas de pedirlo, y cada una necesita nombre.

## Estados que existen y hay que representar

La importación entera:
  leyendo · lista · publicada · fallida

Cada foto:
  sin foto · en cola · generándose · lista · falló · se acabó el presupuesto

Tipos de negocio, que el sistema deduce y el dueño corrige:
  cevicheria · polleria · chifa · comida criolla · parrilla · pizzeria ·
  sangucheria · otro

## Restricciones reales, medidas

  - Cada foto generada cuesta dinero de verdad. El tope por carta alcanza para
    unas 89 fotos. Al agotarse, el dueño puede seguir subiendo las suyas, que no
    gastan.
  - Una foto tarda unos 4 segundos. Se generan cuatro a la vez, así que una carta
    de 74 platos tarda alrededor de un minuto. Van llegando de a pocas, no todas
    juntas, y el dueño puede seguir trabajando mientras.
  - Leer la carta tarda unos diez segundos.
  - Las cartas reales tienen entre 40 y 80 platos repartidos en hasta 11
    categorías. Los nombres de plato son largos: "Ceviche + Arroz c/ Mariscos +
    Chicharrón Mixto".
  - Todo esto ocurre en un teléfono con datos móviles.

## Lo que nunca puede pasar

  - Que se publique un precio equivocado sin que el dueño lo haya visto.
  - Que una acción cueste dinero sin que el dueño sepa que va a costarlo.
  - Que se pierda una foto que el dueño subió.
  - Que el dueño pierda de vista lo que estaba haciendo porque algo se movió
    solo: las fotos llegan de a una mientras él corrige precios.
  - Que un control esté deshabilitado sin decir por qué.
  - Que el dueño llegue a un punto sin salida del que solo se sale empezando de
    cero.

## Qué tienes que decidir tú

Todo lo visual y lo de interacción: la disposición, la navegación entre pasos,
cómo se representa el progreso, cómo se editan las cosas, qué se ve a la vez y
qué no, jerarquía, densidad, movimiento, y el comportamiento en teléfono frente a
pantalla ancha.

Este documento describe el trabajo, los datos y las restricciones. No prescribe
ninguna forma: si algo de aquí te suena a un componente concreto, ignóralo y
resuélvelo como corresponda.
