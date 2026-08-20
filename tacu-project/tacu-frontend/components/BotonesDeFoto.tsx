"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { generarFotos, quitarFotoDePlato, subirFotoDePlato } from "@/lib/api";
import type { Foto } from "@/lib/tipos";
import { SubirImagen } from "@/components/SubirImagen";
import { Boton } from "./ui/Boton";

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
      etiqueta="Subir"
      onArchivo={async (archivo) => {
        await subirFotoDePlato(idImportacion, idPlato, archivo);
        await refrescar();
      }}
    />
  );

  if (estado === "pendiente" || estado === "generando") {
    return (
      <Boton disabled ancho tamano="sm" variante="fantasma">
        {estado === "pendiente" ? "En cola…" : "Dibujando…"}
      </Boton>
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex gap-2">
        {estado === "lista" ? (
          <>
            <Boton
              className="flex-1"
              disabled={ocupado}
              tamano="sm"
              variante="fantasma"
              onClick={() => quitar.mutate()}
            >
              {quitar.isPending ? "Quitando..." : "Quitar"}
            </Boton>
            <Boton
              className="flex-1"
              disabled={ocupado}
              tamano="sm"
              variante="fantasma"
              onClick={() => generar.mutate()}
            >
              {generar.isPending ? "Pidiendo..." : "Regenerar"}
            </Boton>
          </>
        ) : estado === "error" ? (
          <Boton
            className="flex-1"
            disabled={ocupado}
            tamano="sm"
            variante="fantasma"
            onClick={() => generar.mutate()}
          >
            {generar.isPending ? "Reintentando..." : "Reintentar"}
          </Boton>
        ) : estado === "sin_presupuesto" ? (
          subir
        ) : (
          <>
            {subir}
            <Boton
              className="flex-1"
              disabled={ocupado}
              tamano="sm"
              variante="amarillo"
              onClick={() => generar.mutate()}
            >
              {generar.isPending ? "Pidiendo…" : "Generar"}
            </Boton>
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
