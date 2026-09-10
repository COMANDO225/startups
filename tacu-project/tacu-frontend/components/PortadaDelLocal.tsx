"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ImagePlus, Loader2, Trash2 } from "lucide-react";
import { ponerPortada, quitarPortada, urlMedia } from "@/lib/api";
import type { Portada } from "@/lib/tipos";
import { Aviso } from "./ui/Aviso";
import { useState } from "react";

const TIPOS = ["image/jpeg", "image/png", "image/webp"];
const MAX_MB = 10;

/**
 * La foto del local, la que encabeza el catalogo publico.
 *
 * Es opcional Y SE DICE: sin ella el catalogo sale con el nombre en texto, que
 * es como salia antes de que esto existiera y es una salida digna. Un campo que
 * parece obligatorio en el paso de menos paciencia es fricción por nada.
 */
export function PortadaDelLocal({
  idImportacion,
  portada,
}: {
  idImportacion: string;
  portada: Portada;
}) {
  const cliente = useQueryClient();
  const [aviso, setAviso] = useState<string | null>(null);

  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["importacion", idImportacion] });

  const poner = useMutation({
    mutationFn: (foto: File) => ponerPortada(idImportacion, foto),
    onSuccess: refrescar,
    onError: (e: Error) => setAviso(e.message),
  });
  const quitar = useMutation({
    mutationFn: () => quitarPortada(idImportacion),
    onSuccess: refrescar,
  });

  const ocupado = poner.isPending || quitar.isPending;
  const hay = !!portada?.url;

  function elegir(foto: File | undefined) {
    if (!foto) return;
    if (!TIPOS.includes(foto.type)) {
      setAviso(`"${foto.name}" no es una foto.`);
      return;
    }
    if (foto.size > MAX_MB * 1024 * 1024) {
      setAviso(`La foto pesa más de ${MAX_MB} MB.`);
      return;
    }
    setAviso(null);
    poner.mutate(foto);
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-[13px] font-medium">Foto de tu local (opcional)</p>
      <p className="text-[13.5px] leading-[1.5] text-parrafo">
        Encabeza tu carta cuando la abran. Si no pones ninguna, sale tu nombre.
      </p>

      <div className="mt-1 flex items-start gap-2.5">
        <label
          className={`relative flex aspect-[16/10] w-[188px] shrink-0 cursor-pointer flex-col items-center justify-center gap-1.5 overflow-hidden rounded-xl border text-tenue transition-colors lg:w-[232px] ${
            hay
              ? "border-borde-suave"
              : "border-dashed border-borde-campo bg-surface hover:border-tinta hover:text-tinta"
          }`}
        >
          <input
            accept={TIPOS.join(",")}
            className="sr-only"
            disabled={ocupado}
            type="file"
            onChange={(e) => {
              elegir(e.target.files?.[0]);
              e.target.value = "";
            }}
          />

          {hay ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              alt="La foto de tu local"
              className="size-full object-cover"
              src={urlMedia(portada.url_media ?? portada.url)}
            />
          ) : (
            <>
              <ImagePlus className="size-6" />
              <span className="text-[12.5px]">Subir una foto</span>
            </>
          )}

          {ocupado && (
            <div className="absolute inset-0 grid place-items-center bg-surface/70">
              <Loader2 className="size-5 animate-spin text-tinta" />
            </div>
          )}
        </label>

        {hay && (
          <button
            aria-label="Quitar la foto del local"
            className="grid size-9 place-items-center rounded-[9px] text-tenue transition-colors hover:bg-hueso hover:text-bloquea"
            disabled={ocupado}
            type="button"
            onClick={() => quitar.mutate()}
          >
            <Trash2 className="size-4" />
          </button>
        )}
      </div>

      {aviso && <Aviso tono="bloquea">{aviso}</Aviso>}
    </div>
  );
}
