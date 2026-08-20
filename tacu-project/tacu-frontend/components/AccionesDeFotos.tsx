"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boton } from "./ui/Boton";
import { Girador } from "./ui/Girador";
import { Medidor } from "./ui/Medidor";
import { Palette, Sparkles } from "lucide-react";
import { generarFotos, obtenerEstilo } from "@/lib/api";
import { useFotos } from "@/lib/hooks";

/**
 * La tarjeta medidor: cuantas fotos hay, cuantas faltan y el boton de generar.
 *
 * Vive DENTRO de la pantalla de fotos y solo ahi. Un boton de generar mientras
 * se revisan precios no es que estorbe: invita a gastar antes de comprobar los
 * datos, que es el orden que este flujo existe para evitar.
 */
export function AccionesDeFotos({
  idImportacion,
  onEstilo,
}: {
  idImportacion: string;
  onEstilo: () => void;
}) {
  const cliente = useQueryClient();
  const { data: fotos } = useFotos(idImportacion);
  const { data: estilo } = useQuery({
    queryKey: ["estilo", idImportacion, ""],
    queryFn: () => obtenerEstilo(idImportacion, ""),
  });

  const generarTodas = useMutation({
    mutationFn: () => generarFotos(idImportacion, { todos: true }),
    // El poll se apaga cuando pendientes llega a 0, o sea justo antes de
    // encolar: sin este invalidate el avance no arranca hasta el siguiente.
    onSuccess: () =>
      cliente.invalidateQueries({ queryKey: ["fotos", idImportacion] }),
  });

  const lista = Object.values(fotos?.fotos ?? {});
  const enCurso = fotos?.pendientes ?? 0;
  const listas = lista.filter((f) => f.estado === "lista").length;
  const gasto = fotos?.gasto;
  const caben =
    gasto && gasto.por_foto_usd > 0
      ? Math.floor(
          Math.max(0, gasto.presupuesto_usd - gasto.gastado_usd) /
            gasto.por_foto_usd,
        )
      : 0;

  const estiloPropio = !!(estilo?.vajilla.tocada || estilo?.fondo.tocada);

  return (
    <div className="mt-4 rounded-[14px] border border-border bg-surface px-4 py-3.5">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:gap-3.5">
        <div className="min-w-0 flex-1">
          <p className="text-[13px] font-medium leading-none">
            <span className="font-display tabular-nums">{listas}</span> de{" "}
            <span className="font-display tabular-nums">{lista.length}</span>{" "}
            platos con foto
          </p>
          <div className="mt-2.5">
            <Medidor de={listas} sobre={lista.length} />
          </div>
        </div>

        {enCurso > 0 ? (
          <span className="flex shrink-0 items-center gap-2 text-[11.5px] text-tenue">
            <Girador tam={11} />
            van llegando de a pocas
          </span>
        ) : (
          caben > 0 && (
            <Boton
              ancho
              className="shrink-0 lg:w-auto"
              disabled={generarTodas.isPending}
              tamano="sm"
              variante="amarillo"
              onClick={() => generarTodas.mutate()}
            >
              <Sparkles className="size-4" />
              {generarTodas.isPending ? "Encolando…" : "Generar las que faltan"}
            </Boton>
          )
        )}
      </div>

      {gasto && (
        <p className="mt-2.5 text-[11.5px] leading-[1.4] text-tenue">
          {caben === 0
            ? "Se acabaron las fotos con IA. Las tuyas no gastan nada."
            : `Te queda${caben === 1 ? "" : "n"} ~${caben} foto${caben === 1 ? "" : "s"} con IA`}
        </p>
      )}

      {/* En el telefono el estilo no cabe en la cabecera: va aqui, separado. */}
      <button
        className="mt-3 flex w-full items-center gap-2 border-t border-separator pt-3 text-[12.5px] text-muted transition-colors hover:text-tinta lg:hidden"
        type="button"
        onClick={onEstilo}
      >
        <Palette className="size-4" />
        Estilo de las fotos
        {estiloPropio && <span className="size-1.5 rounded-full bg-accent" />}
      </button>
    </div>
  );
}
