"use client";

import { useSyncExternalStore } from "react";

/**
 * Los dos cortes de ancho, leidos como lo que son: un store externo.
 *
 * useSyncExternalStore y no useEffect: en el servidor no hay matchMedia, y el
 * tercer argumento da el valor del render de servidor sin que salte el aviso de
 * hidratacion. Ademas evita el parpadeo de montar el dialogo equivocado y
 * cambiarlo, que en un dialogo significa perder el foco.
 */
const ESCRITORIO = "(min-width: 640px)"; // el sm: de Tailwind
const ANCHA = "(min-width: 1024px)"; // el lg: donde el rail sustituye a la cabecera

function suscribirA(consulta: string) {
  return (avisar: () => void) => {
    const mq = window.matchMedia(consulta);
    mq.addEventListener("change", avisar);
    return () => mq.removeEventListener("change", avisar);
  };
}

const aEscritorio = suscribirA(ESCRITORIO);
const aAncha = suscribirA(ANCHA);

/** Decide entre modal y drawer. */
export function useEsEscritorio(): boolean {
  return useSyncExternalStore(
    aEscritorio,
    () => window.matchMedia(ESCRITORIO).matches,
    () => true, // en el servidor se asume ancha: es lo que menos salta
  );
}

/**
 * Decide si el marco es el rail o la cabecera de telefono.
 *
 * Hace falta como HOOK y no solo como clase `lg:` para lo que se monta de
 * verdad: la barra de accion se cuelga del body por un portal, y en el servidor
 * no hay body. Con la clase se renderizaria igual y solo quedaria escondida.
 */
export function useEsAncha(): boolean {
  return useSyncExternalStore(
    aAncha,
    () => window.matchMedia(ANCHA).matches,
    () => true,
  );
}
