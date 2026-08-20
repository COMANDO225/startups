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

  // El compacto es el boton fantasma del diseno, con el area de toque de
  // telefono: por debajo de 40 px el dedo falla.
  const forma = compacto
    ? "min-h-10 flex-1 flex-row gap-1.5 whitespace-nowrap rounded-[9px] bg-hueso px-[15px] text-[12.5px] font-medium text-tinta hover:brightness-[0.98] lg:min-h-[34px]"
    : "size-20 flex-col gap-1 rounded-xl border border-dashed border-borde-campo text-[10px] leading-tight text-tenue hover:border-tinta hover:text-tinta";

  return (
    <label
      className={`flex cursor-pointer items-center justify-center text-center ${forma} ${
        subiendo ? "opacity-60" : ""
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
