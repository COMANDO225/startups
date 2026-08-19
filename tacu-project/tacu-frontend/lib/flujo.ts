import type { Importacion } from "./tipos";

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
  /** Bloqueada: todavia no hay carta que revisar. */
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
      bloqueada: false,
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
      bloqueada: !leida,
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
      bloqueada: !leida,
      subs: [],
    },
  ];
}
