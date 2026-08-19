"use client";

import { useState } from "react";
import { ImagePlus } from "lucide-react";

/**
 * Subida de archivo con <label> + input sr-only.
 *
 * HeroUI v3 no trae FileTrigger, y este patron ya funciona: sr-only y no hidden
 * porque hidden saca el input del foco del teclado.
 */
export function SubirImagen({
  onArchivo,
  etiqueta = "Subir foto",
  compacto = false,
}: {
  onArchivo: (archivo: File) => Promise<void>;
  etiqueta?: string;
  /** compacto = botón en línea; si no, cuadro cuadrado para las referencias. */
  compacto?: boolean;
}) {
  const [subiendo, setSubiendo] = useState(false);
  const [fallo, setFallo] = useState<string | null>(null);

  const forma = compacto
    ? "flex-1 h-8 flex-row gap-1.5 rounded-lg border border-default px-3 text-sm hover:bg-surface-secondary"
    : "size-20 flex-col gap-1 rounded-lg border border-dashed border-default text-[10px] leading-tight hover:border-foreground";

  return (
    <label
      className={`flex cursor-pointer items-center justify-center text-center text-muted ${forma} ${
        subiendo ? "animate-pulse" : ""
      }`}
      title={fallo ?? etiqueta}
    >
      <ImagePlus className="size-4 shrink-0" />
      {subiendo ? "Subiendo" : etiqueta}
      <input
        accept="image/*"
        className="sr-only"
        disabled={subiendo}
        type="file"
        onChange={async (e) => {
          const archivo = e.target.files?.[0];
          e.target.value = ""; // elegir el MISMO archivo otra vez vuelve a disparar
          if (!archivo) return;
          setSubiendo(true);
          setFallo(null);
          try {
            await onArchivo(archivo);
          } catch (err) {
            setFallo(err instanceof Error ? err.message : "No se pudo subir");
          } finally {
            setSubiendo(false);
          }
        }}
      />
    </label>
  );
}
