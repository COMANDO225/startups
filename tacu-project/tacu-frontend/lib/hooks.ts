"use client";

import { useSyncExternalStore } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  escucharRestaurantes,
  FalloApi,
  misRestaurantes,
  obtenerFotos,
  obtenerImportacion,
} from "./api";
import type { EstadoFotos, Foto, Importacion } from "./tipos";

// Un 401/404/400 no mejora reintentando: es el token, el id o la peticion.
function reintentar(veces: number, error: Error) {
  if (error instanceof FalloApi) return false;
  return veces < 2;
}

/** Poll mientras la IA lee la carta. Se apaga solo cuando deja de estar "leyendo". */
export function useImportacion(id: string | undefined) {
  return useQuery<Importacion>({
    queryKey: ["importacion", id],
    queryFn: () => obtenerImportacion(id!),
    enabled: !!id,
    retry: reintentar,
    // Solo mientras lee. Es la unica etapa que cambia sola; el resto lo mueve
    // el dueno y ya invalida la consulta al hacerlo.
    refetchInterval: (q) => (q.state.data?.estado === "leyendo" ? 1500 : false),
  });
}

/**
 * Poll del minuto de las fotos. Devuelve el mapa por id de plato TAL CUAL: cada
 * tarjeta lee solo su entrada, para que un poll no repinte el catalogo entero y
 * le robe el foco al dueno mientras edita un precio.
 */
export function useFotos(id: string | undefined, activo = true) {
  return useQuery<EstadoFotos>({
    queryKey: ["fotos", id],
    queryFn: () => obtenerFotos(id!),
    enabled: !!id && activo,
    retry: reintentar,
    refetchInterval: (q) =>
      (q.state.data?.pendientes ?? 0) > 0 ? 2000 : false,
  });
}

/**
 * La foto de UN plato. Es lo que usa cada tarjeta.
 *
 * Existe por el `select`: sin el, las N tarjetas se suscriben al objeto completo
 * del poll y las N vuelven a renderizar cada vez que UNA foto cambia. Con el,
 * React Query compara solo la entrada de ese plato y la tarjeta se entera unicamente
 * cuando cambia LA SUYA.
 *
 * Con 60 platos y 60 fotos llegando, la diferencia son 60 renders contra 3600.
 */
export function useFotoDePlato(
  idImportacion: string | undefined,
  idPlato: string,
) {
  return useQuery<EstadoFotos, Error, Foto | undefined>({
    queryKey: ["fotos", idImportacion],
    queryFn: () => obtenerFotos(idImportacion!),
    enabled: !!idImportacion,
    retry: reintentar,
    refetchInterval: (q) =>
      (q.state.data?.pendientes ?? 0) > 0 ? 2000 : false,
    select: (d) => d.fotos[idPlato],
  });
}

/** Cuantas fotos faltan y cuanto se lleva gastado. Para la barra de progreso. */
export function useAvanceDeFotos(id: string | undefined) {
  return useQuery<
    EstadoFotos,
    Error,
    {
      pendientes: number;
      listas: number;
      total: number;
      gasto: EstadoFotos["gasto"];
    }
  >({
    queryKey: ["fotos", id],
    queryFn: () => obtenerFotos(id!),
    enabled: !!id,
    retry: reintentar,
    refetchInterval: (q) =>
      (q.state.data?.pendientes ?? 0) > 0 ? 2000 : false,
    select: (d) => {
      const fotos = Object.values(d.fotos);
      return {
        pendientes: d.pendientes,
        listas: fotos.filter((f) => f.estado === "lista").length,
        total: fotos.length,
        gasto: d.gasto,
      };
    },
  });
}

/**
 * Los restaurantes de este navegador.
 *
 * localStorage es un store externo, asi que se lee con useSyncExternalStore y
 * no con un efecto: en el servidor no existe, y leerlo durante el render
 * desharia la hidratacion.
 */
export function useMisRestaurantes() {
  // La MISMA funcion para las dos instantaneas: misRestaurantes ya devuelve la
  // constante vacia cuando no hay window, y esa es justo la parte que tiene que
  // ser estable. Un `() => []` como tercer argumento crea un array nuevo en cada
  // llamada, React lo compara por identidad, y eso es un bucle infinito.
  return useSyncExternalStore(
    escucharRestaurantes,
    misRestaurantes,
    misRestaurantes,
  );
}
