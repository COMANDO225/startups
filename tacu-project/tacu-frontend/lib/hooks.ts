"use client";

import { useSyncExternalStore } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  escucharCarta,
  FalloApi,
  miRestaurante,
  obtenerFotos,
  obtenerImportacion,
  sinRestaurante,
} from "./api";
import type { EstadoFotos, Foto, Importacion } from "./tipos";

// Un 401/404/400 no mejora reintentando: es el token, el id o la peticion.
function reintentar(veces: number, error: Error) {
  if (error instanceof FalloApi) return false;
  return veces < 2;
}

/**
 * Cada cuanto se rehacen las URLs firmadas de lo privado —las hojas de la carta,
 * el estilo, las fotos de ejemplo—, que viven una hora.
 *
 * React Query pausa el intervalo cuando la pestana no esta a la vista, asi que
 * esto no genera trafico de fondo: al volver, quien refresca es el foco.
 */
const REFRESCO_DE_URLS = 25 * 60 * 1000;

/** Poll mientras la IA lee la carta. Se apaga solo cuando deja de estar "leyendo". */
export function useImportacion(id: string | undefined) {
  return useQuery<Importacion>({
    queryKey: ["importacion", id],
    queryFn: () => obtenerImportacion(id!),
    enabled: !!id,
    retry: reintentar,
    // Rapido mientras lee, que es la unica etapa que cambia sola. Despues no se
    // apaga del todo: esta respuesta trae las URLs firmadas de las hojas y hay
    // que renovarlas antes de que caduquen, o una sesion larga de edicion
    // termina con el mosaico en imagenes rotas.
    refetchInterval: (q) =>
      q.state.data?.estado === "leyendo" ? 1500 : REFRESCO_DE_URLS,
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
 * La carta de este navegador, que es UNA. Ver miRestaurante en api.ts.
 *
 * localStorage es un store externo, asi que se lee con useSyncExternalStore y
 * no con un efecto: en el servidor no existe, y leerlo durante el render
 * desharia la hidratacion.
 */
// Un store que no cambia nunca: lo unico que interesa es que su instantanea de
// servidor y la de cliente sean distintas.
const nadaQueEscuchar = () => () => {};

/**
 * Si ya se leyo el navegador.
 *
 * En el servidor —y en el render de hidratacion, que corre antes de que React
 * pida la instantanea del cliente— es false. Hace falta para distinguir "no hay
 * carta" de "todavia no lo se": las dos dan null, y confundirlas le ensena el
 * formulario de crear a quien ya tiene la suya.
 */
export function useYaLeido() {
  return useSyncExternalStore(
    nadaQueEscuchar,
    () => true,
    () => false,
  );
}

export function useMiRestaurante() {
  // La tercera es sinRestaurante y NO miRestaurante: React usa esa tambien en
  // el render de hidratacion, y ahi window ya existe. Ver sinRestaurante.
  return useSyncExternalStore(escucharCarta, miRestaurante, sinRestaurante);
}
