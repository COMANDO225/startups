"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, ProgressBar } from "@heroui/react";
import { Palette, Sparkles } from "lucide-react";
import { generarFotos, obtenerEstilo } from "@/lib/api";
import { useFotos } from "@/lib/hooks";

/**
 * Lo que se puede hacer ESTANDO en las fotos, y solo ahi.
 *
 * Un boton de generar mientras se revisan precios no es que estorbe: invita a
 * gastar antes de comprobar los datos, que es el orden que este flujo existe
 * para evitar.
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

  const estiloPropio = !!(estilo?.base.recipiente || estilo?.base.fondo);

  return (
    <div className="flex flex-col gap-2 border-t border-border pt-4">
      <Button
        className="relative justify-start"
        size="sm"
        variant="tertiary"
        onPress={onEstilo}
      >
        <Palette className="size-4" />
        Estilo de las fotos
        {estiloPropio && (
          <span className="absolute end-2 top-2 size-1.5 rounded-full bg-accent" />
        )}
      </Button>

      {enCurso > 0 ? (
        <div className="flex flex-col gap-1.5 px-2">
          <ProgressBar
            aria-label="Fotos generadas"
            maxValue={lista.length}
            value={listas}
          />
          <span className="text-xs tabular-nums text-muted">
            {listas} de {lista.length} · van llegando de a pocas
          </span>
        </div>
      ) : (
        caben > 0 && (
          <Button
            isDisabled={generarTodas.isPending}
            size="sm"
            onPress={() => generarTodas.mutate()}
          >
            <Sparkles className="size-4" />
            {generarTodas.isPending ? "Encolando..." : "Generar las que faltan"}
          </Button>
        )
      )}

      {gasto && (
        <p className="px-2 text-xs text-muted">
          {caben === 0
            ? "Se acabaron las fotos con IA. Las tuyas no gastan nada."
            : `Te queda${caben === 1 ? "" : "n"} ~${caben} foto${caben === 1 ? "" : "s"} con IA`}
        </p>
      )}
    </div>
  );
}
