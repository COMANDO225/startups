import type { Importacion } from "./tipos";

/**
 * LOS PASOS, que ahora son rutas: /i/{id}/{paso}.
 *
 * El orden del array es el del recorrido y de el sale todo lo demas.
 */
export const PASOS = [
  "datos",
  "carta",
  "revisar",
  "fotos",
  "publicar",
] as const;

export type Paso = (typeof PASOS)[number];

/** Donde vive cada paso dentro de las tres secciones de la columna. */
export const DONDE: Record<Paso, { seccion: string; sub: string }> = {
  datos: { seccion: "restaurante", sub: "datos" },
  carta: { seccion: "restaurante", sub: "carta" },
  revisar: { seccion: "carta", sub: "revisar" },
  fotos: { seccion: "carta", sub: "fotos" },
  publicar: { seccion: "publicar", sub: "" },
};

export function pasoValido(s: string | null): Paso | null {
  return PASOS.includes(s as Paso) ? (s as Paso) : null;
}

/**
 * LA REGLA, y la unica. La consumen los tres: el stepper para pintar el
 * candado, la navegacion para no ir, y el guard de ruta para redirigir.
 *
 * Antes vivia escrita tres veces —dos `bloqueada: !leida` y un `sinCarta` que
 * forzaba la seccion desde la pagina— con criterios parecidos pero no iguales.
 * Cada estado nuevo (paso con 'nueva' y con 'fallida') habia que acordarse de
 * los tres sitios.
 *
 * Sale del estado REAL de la carta, no de banderas de pantalla.
 */
export function puedeIr(paso: Paso, imp?: Importacion): boolean {
  switch (paso) {
    // El principio: siempre se puede volver a los datos.
    case "datos":
      return true;

    // Entregar las hojas necesita que el restaurante exista.
    case "carta":
      return !!imp;

    // Revisar, las fotos y publicar necesitan platos. Con 'nueva', 'leyendo' o
    // 'fallida' no hay uno solo que ensenar, y una pantalla vacia con un
    // candado invisible es un punto muerto.
    default:
      return imp?.estado === "lista" || imp?.estado === "publicada";
  }
}

/**
 * Donde aterrizar cuando no se pide un paso concreto, o cuando el pedido no se
 * puede: el mas avanzado que esta carta permite.
 *
 * Es la otra mitad de la regla: si hay futuro es porque hay un pasado, asi que
 * volver a /i/{id} despues de leer la carta no puede devolverte al paso 1.
 */
export function alcance(imp?: Importacion): Paso {
  if (!imp) return "datos";
  if (imp.estado === "publicada") return "publicar";
  if (puedeIr("revisar", imp)) return "revisar";
  return "carta";
}


/**
 * El flujo son TRES SECCIONES con sub-pasos, no una lista plana de pasos.
 *
 * La distincion que lo ordena: un PASO lo recorre el dueno y se navega; una
 * ETAPA la ejecuta el sistema dentro de un paso y no se navega — leer la carta,
 * cruzar los precios, deducir el tipo. Antes tenia las etapas mezcladas con los
 * pasos y por eso ninguna jerarquia cuadraba.
 */
export type SubPaso = {
  id: string;
  label: string;
  listo: boolean;
  /** Contador al lado. rojo = bloquea publicar. */
  tag?: string;
  rojo?: boolean;
  /** El sistema esta trabajando en este sub-paso ahora mismo. */
  ocupado?: boolean;
};

export type Seccion = {
  id: string;
  num: string;
  titulo: string;
  /** Version de una palabra, para cuando no cabe el titulo. */
  corto: string;
  listo: boolean;
  /** Bloqueada: la regla dice que esta carta todavia no llega aqui. */
  bloqueada: boolean;
  subs: SubPaso[];
};

/**
 * Las etapas de la lectura. El backend manda el indice; los textos son de aqui.
 *
 * EL ORDEN ES EL CONTRATO: la pantalla da por hechas todas las anteriores al
 * indice que recibe, asi que este array tiene que ir en el mismo orden que las
 * constantes de domain/importacion.go y que las llamadas del worker. Cuando no
 * coincidian, la lista marcaba como reconocido el tipo de restaurante mientras
 * el sistema todavia estaba ordenando.
 *
 * Y cada frase dice lo que de verdad esta pasando. Es lo unico que el dueno mira
 * mientras espera, asi que es donde se gana o se pierde su confianza: "cruzando
 * cada precio con el texto impreso" vende porque es exactamente su miedo, y
 * porque es verdad.
 */
export const ETAPAS = [
  "Revisando tus hojas",
  "Leyendo platos, precios y categorías",
  "Cruzando cada precio con el texto impreso",
  "Ordenando tus categorías",
  "Reconociendo qué es cada plato",
  "Reconociendo qué tipo de restaurante eres",
  "Guardando",
] as const;

export function secciones({
  importacion,
  conFoto,
  platos,
}: {
  importacion?: Importacion;
  conFoto: number;
  platos: number;
}): Seccion[] {
  // 'nueva' es el restaurante creado y todavia sin hojas; 'fallida' es la carta
  // que no se pudo leer. Las dos cuentan como NO leida: sin platos no hay
  // precios que revisar ni fotos que generar, y dejar el paso 2 abierto manda
  // al dueno a una pantalla vacia — que es justo lo que pasaba con fallida.
  const leida =
    !!importacion &&
    importacion.estado !== "leyendo" &&
    importacion.estado !== "nueva" &&
    importacion.estado !== "fallida";
  const leyendo = importacion?.estado === "leyendo";
  const bloquean = importacion?.marcas.revisar ?? 0;
  const confirmar = importacion?.marcas.confirmar ?? 0;

  return [
    {
      id: "restaurante",
      num: "1",
      titulo: "Tu restaurante",
      corto: "Datos",
      listo: leida,
      bloqueada: !puedeIr("datos", importacion),
      subs: [
        {
          id: "datos",
          label: "Tus datos",
          listo: !!importacion?.restaurante.nombre,
        },
        // Las hojas que entrego. Ya no hace falta llamarlas "de papel" para
        // distinguirlas del paso 2: ese paso ahora se llama "Tu catalogo",
        // que es lo que de verdad es.
        {
          id: "carta",
          label: "Tu carta",
          listo: leida,
          ocupado: leyendo,
        },
      ],
    },
    {
      id: "carta",
      num: "2",
      // "Tu catalogo" y no "Tu carta": la carta es el papel que entrego y vive
      // en el paso 1. Esto es lo que le hicimos con ella, que es el producto.
      titulo: "Tu catálogo",
      corto: "Catálogo",
      listo: leida && bloquean === 0,
      bloqueada: !puedeIr("revisar", importacion),
      subs: [
        {
          id: "revisar",
          label: "Revisar precios",
          listo: bloquean === 0 && confirmar === 0,
          // El contador rojo es el que bloquea publicar; el gris solo pide una
          // mirada. Pintarlos igual entrena a aprobar sin leer.
          tag: bloquean ? String(bloquean) : confirmar ? String(confirmar) : "",
          rojo: bloquean > 0,
        },
        {
          id: "fotos",
          label: "Las fotos",
          listo: platos > 0 && conFoto === platos,
          tag: platos ? `${conFoto}/${platos}` : "",
        },
      ],
    },
    {
      id: "publicar",
      num: "3",
      titulo: "Publicar",
      corto: "Publicar",
      listo: importacion?.estado === "publicada",
      bloqueada: !puedeIr("publicar", importacion),
      subs: [],
    },
  ];
}
