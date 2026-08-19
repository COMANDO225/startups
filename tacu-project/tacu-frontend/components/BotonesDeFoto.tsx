"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@heroui/react";
import { generarFotos, quitarFotoDePlato, subirFotoDePlato } from "@/lib/api";
import type { Foto } from "@/lib/tipos";
import { SubirImagen } from "@/components/SubirImagen";

/**
 * La foto llega por prop desde el mapa de useFotos: la tarjeta lee SOLO su
 * entrada y se parchea sola, sin repintar el catalogo en cada poll.
 */
export function BotonesDeFoto({
  idImportacion,
  idPlato,
  foto,
}: {
  idImportacion: string;
  idPlato: string;
  foto?: Foto;
}) {
  const cliente = useQueryClient();
  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["fotos", idImportacion] });

  const generar = useMutation({
    mutationFn: () => generarFotos(idImportacion, { platos: [idPlato] }),
    onSuccess: refrescar,
  });
  const quitar = useMutation({
    mutationFn: () => quitarFotoDePlato(idImportacion, idPlato),
    onSuccess: refrescar,
  });

  const ocupado = generar.isPending || quitar.isPending;
  const estado = foto?.estado ?? "vacia";
  const fallo = generar.error ?? quitar.error;

  const subir = (
    <SubirImagen
      compacto
      etiqueta="Subir imagen"
      onArchivo={async (archivo) => {
        await subirFotoDePlato(idImportacion, idPlato, archivo);
        await refrescar();
      }}
    />
  );

  if (estado === "pendiente" || estado === "generando") {
    return (
      <Button isDisabled className="w-full" size="sm" variant="secondary">
        {estado === "pendiente" ? "En cola..." : "Dibujando..."}
      </Button>
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex gap-2">
        {estado === "lista" ? (
          <>
            <Button
              className="flex-1"
              isDisabled={ocupado}
              size="sm"
              variant="secondary"
              onPress={() => quitar.mutate()}
            >
              {quitar.isPending ? "Quitando..." : "Quitar"}
            </Button>
            <Button
              className="flex-1"
              isDisabled={ocupado}
              size="sm"
              variant="secondary"
              onPress={() => generar.mutate()}
            >
              {generar.isPending ? "Pidiendo..." : "Regenerar"}
            </Button>
          </>
        ) : estado === "error" ? (
          <Button
            className="flex-1"
            isDisabled={ocupado}
            size="sm"
            variant="secondary"
            onPress={() => generar.mutate()}
          >
            {generar.isPending ? "Reintentando..." : "Reintentar"}
          </Button>
        ) : estado === "sin_presupuesto" ? (
          subir
        ) : (
          <>
            {subir}
            <Button
              className="flex-1"
              isDisabled={ocupado}
              size="sm"
              onPress={() => generar.mutate()}
            >
              {generar.isPending ? "Pidiendo..." : "Generar con IA"}
            </Button>
          </>
        )}
      </div>

      {estado === "sin_presupuesto" && (
        <p className="text-xs text-muted">
          Ya usaste todas las fotos con IA de esta carta. Sube una foto tuya.
        </p>
      )}

      {fallo && <p className="text-xs text-bloquea">{fallo.message}</p>}
    </div>
  );
}
