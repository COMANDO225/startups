"use client";

import { useSyncExternalStore } from "react";

/**
 * useEsEscritorio decide entre modal y drawer.
 *
 * useSyncExternalStore y no useEffect: en el servidor no hay matchMedia, y el
 * tercer argumento da el valor del render servidor sin que salte el aviso de
 * hidratacion. Ademas evita el parpadeo de montar el dialogo equivocado y
 * cambiarlo, que en un dialogo significa perder el foco.
 */
const CONSULTA = "(min-width: 640px)"; // el sm: de Tailwind

function suscribir(avisar: () => void) {
  const mq = window.matchMedia(CONSULTA);
  mq.addEventListener("change", avisar);
  return () => mq.removeEventListener("change", avisar);
}

export function useEsEscritorio(): boolean {
  return useSyncExternalStore(
    suscribir,
    () => window.matchMedia(CONSULTA).matches,
    () => true, // en el servidor se asume escritorio: es lo que menos salta
  );
}
