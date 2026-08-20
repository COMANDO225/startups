"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { ImageOff, Trash2, Undo2 } from "lucide-react";
import { quitarPlato, recuperarPlato, urlMedia } from "@/lib/api";
import type { Plato } from "@/lib/tipos";
import { Boton } from "./ui/Boton";

/**
 * Los platos que la ultima lectura ya no trajo.
 *
 * NO se borran solos, y ese es todo el punto: un plato puede tener una foto de
 * $0.0336 detras, y la lectura se pudo equivocar —una hoja movida, un reflejo
 * sobre el plastico— asi que quien decide es el dueno. Aqui estan a la vista
 * con las dos salidas: se fue de la carta, o sigue y la lectura fallo.
 */
export function PlatosAusentes({
  idImportacion,
  platos,
}: {
  idImportacion: string;
  platos: Plato[];
}) {
  const cliente = useQueryClient();
  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["importacion", idImportacion] });

  const quitar = useMutation({
    mutationFn: (idPlato: string) => quitarPlato(idImportacion, idPlato),
    onSuccess: refrescar,
  });
  const recuperar = useMutation({
    mutationFn: (idPlato: string) => recuperarPlato(idImportacion, idPlato),
    onSuccess: refrescar,
  });

  if (platos.length === 0) return null;

  const conFoto = platos.filter((p) => p.foto?.url).length;

  return (
    <section className="mb-6 flex flex-col gap-3 rounded-2xl border border-border bg-surface p-4">
      <div>
        <h2 className="font-display text-[15px] font-semibold leading-[1.25]">
          {platos.length === 1
            ? "Un plato ya no aparece en tu carta"
            : `${platos.length} platos ya no aparecen en tu carta`}
        </h2>
        <p className="mt-1 text-[12.5px] leading-[1.45] text-tenue">
          La última lectura no los encontró. No los borramos por nuestra cuenta
          {conFoto > 0 && (
            <>
              {" "}
              —{conFoto} {conFoto === 1 ? "tiene" : "tienen"} foto ya generada—
            </>
          )}
          : dinos tú si se fueron o si la lectura falló.
        </p>
      </div>

      <ul className="flex flex-col divide-y divide-border">
        <AnimatePresence initial={false}>
          {platos.map((plato) => (
            <motion.li
              key={plato.id}
              animate={{ opacity: 1, height: "auto" }}
              className="flex items-center gap-3 overflow-hidden py-2"
              exit={{ opacity: 0, height: 0 }}
              initial={{ opacity: 0, height: 0 }}
            >
              <div className="size-11 shrink-0 overflow-hidden rounded-lg border border-border bg-surface-secondary">
                {plato.foto?.url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    alt=""
                    className="size-full object-cover"
                    src={urlMedia(plato.foto.url_media ?? plato.foto.url)}
                  />
                ) : (
                  <div className="grid size-full place-items-center text-muted">
                    <ImageOff className="size-4" />
                  </div>
                )}
              </div>

              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{plato.nombre}</p>
                <p className="text-xs text-muted">{plato.desde}</p>
              </div>

              <Boton
                disabled={recuperar.isPending}
                tamano="sm"
                variante="blanco"
                onClick={() => recuperar.mutate(plato.id)}
              >
                <Undo2 className="size-3.5" />
                Sigue en mi carta
              </Boton>
              <Boton
                aria-label={`Quitar ${plato.nombre}`}
                disabled={quitar.isPending}
                tamano="sm"
                variante="fantasma"
                onClick={() => quitar.mutate(plato.id)}
              >
                <Trash2 className="size-3.5" />
                Quitar
              </Boton>
            </motion.li>
          ))}
        </AnimatePresence>
      </ul>
    </section>
  );
}
