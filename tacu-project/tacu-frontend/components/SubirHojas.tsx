"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { FileText, Plus, X } from "lucide-react";
import { subirCarta } from "@/lib/api";
import { Aviso } from "./ui/Aviso";
import { Boton } from "./ui/Boton";
import { Girador } from "./ui/Girador";
import { BarraAccion } from "./BarraAccion";

const TIPOS = ["image/jpeg", "image/png", "image/webp", "application/pdf"];
const MAXIMO_HOJAS = 4;
const MAXIMO_BYTES = 40 * 1024 * 1024;

function mb(bytes: number) {
  return (bytes / 1024 / 1024).toFixed(1).replace(".", ",");
}

/**
 * Las hojas de la carta, cuando el restaurante YA existe.
 *
 * Es la MISMA ficha que Paginas en VistaCarta —mismo tamano, misma etiqueta,
 * mismo boton de quitar al pasar por encima— y a proposito: el dueno ve el
 * mismo objeto antes y despues de leer la carta. Lo que cambia por dentro es
 * que aqui las hojas todavia no existen en el servidor, asi que se acumulan y
 * viajan juntas.
 *
 * Anadir vive en una ficha AL LADO de las que ya subio, no en una caja aparte
 * encima: con la caja separada, la zona mas grande de la pantalla es la que ya
 * no hace falta en cuanto hay una hoja puesta.
 */
export function SubirHojas({ idImportacion }: { idImportacion: string }) {
  const cliente = useQueryClient();
  const entrada = useRef<HTMLInputElement>(null);
  const [archivos, setArchivos] = useState<File[]>([]);
  const [encima, setEncima] = useState(false);
  const [aviso, setAviso] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);

  // Un PDF no se previsualiza: se muestra su icono. Por eso el null.
  const vistas = useMemo(
    () =>
      archivos.map((a) =>
        a.type === "application/pdf" ? null : URL.createObjectURL(a),
      ),
    [archivos],
  );
  useEffect(
    () => () => {
      for (const v of vistas) if (v) URL.revokeObjectURL(v);
    },
    [vistas],
  );

  // Se valida ANTES de mandar y no despues del 400: en un movil con datos,
  // subir 40 MB para que el backend diga que no era una imagen son varios
  // minutos tirados.
  function revisar(juntas: File[]): string | null {
    if (juntas.length > MAXIMO_HOJAS) {
      return `Como máximo ${MAXIMO_HOJAS} hojas. Elegiste ${juntas.length}.`;
    }
    const mala = juntas.find((a) => !TIPOS.includes(a.type));
    if (mala) {
      return `"${mala.name}" no es una foto ni un PDF. Acepta JPG, PNG, WEBP o PDF.`;
    }
    const total = juntas.reduce((suma, a) => suma + a.size, 0);
    if (total > MAXIMO_BYTES) {
      return `Todo junto pesa ${mb(total)} MB y el máximo son 40 MB.`;
    }
    return null;
  }

  function agregar(nuevas: FileList | null) {
    if (!nuevas || nuevas.length === 0) return;
    const juntas = [...archivos, ...Array.from(nuevas)];
    const problema = revisar(juntas);
    setAviso(problema);
    if (!problema) setArchivos(juntas);
  }

  async function enviar(evento?: React.FormEvent) {
    evento?.preventDefault();
    if (enviando || archivos.length === 0) return;
    setAviso(null);
    setEnviando(true);
    try {
      await subirCarta(idImportacion, archivos);
      // No se navega: ya estamos en el paso que toca. Lo que cambia es el
      // estado de la importacion, y al refrescarla esta misma vista pasa sola
      // a enseñar las etapas de la lectura.
      await cliente.invalidateQueries({
        queryKey: ["importacion", idImportacion],
      });
    } catch (error) {
      setAviso(
        error instanceof Error
          ? error.message
          : "No se pudo subir la carta. Intenta otra vez.",
      );
      setEnviando(false);
    }
  }

  const puedeAnadir = archivos.length < MAXIMO_HOJAS && !enviando;

  return (
    <form className="flex flex-col gap-5" onSubmit={enviar}>
      {/* Soltar vale sobre TODA la zona y no solo sobre la ficha de anadir: al
          arrastrar, el dedo o el raton apunta al hueco, no a un boton.

          La marca de "suelta aqui" es un OUTLINE y no un borde con padding: el
          padding metia las fichas 18 px hacia dentro y las dejaba desalineadas
          del titulo y del boton, que son los que marcan el borde de la columna.
          El outline no ocupa sitio, asi que se pinta por fuera sin mover nada. */}
      <div
        className={`flex flex-wrap items-start gap-2.5 rounded-2xl transition-all ${
          encima
            ? "outline-2 outline-offset-8 outline-dashed outline-accent"
            : "outline-transparent"
        }`}
        onDragLeave={() => setEncima(false)}
        onDragOver={(e) => {
          e.preventDefault();
          setEncima(true);
        }}
        onDrop={(e) => {
          e.preventDefault();
          setEncima(false);
          agregar(e.dataTransfer.files);
        }}
      >
        <AnimatePresence initial={false}>
          {archivos.map((archivo, i) => (
            // Sin `layout` y sin escala al entrar: las dos usan transform, se
            // pisan, y la ficha se quedaba clavada en scale(0.9) — 115x144 en
            // vez de 128x160, o sea desalineada del titulo y del boton. La
            // opacidad no toca la caja, asi que la ficha mide siempre lo mismo.
            <motion.div
              key={`${archivo.name}-${archivo.size}-${i}`}
              animate={{ opacity: 1 }}
              className="group relative aspect-[3/4] w-[104px] shrink-0 overflow-hidden rounded-xl border border-[#E7E5E0] bg-surface-secondary"
              exit={{ opacity: 0 }}
              initial={{ opacity: 0 }}
            >
              {vistas[i] ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  alt={`Hoja ${i + 1}`}
                  className="size-full object-cover"
                  draggable={false}
                  src={vistas[i]!}
                />
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-2 px-2 text-center text-muted">
                  <FileText className="size-8" />
                  <span className="line-clamp-2 text-xs">{archivo.name}</span>
                </div>
              )}

              <div className="pointer-events-none absolute inset-x-0 bottom-0 px-2 pb-2 [text-shadow:0_1px_3px_rgba(0,0,0,.55)]">
                <span className="text-xs font-medium text-white">
                  Hoja {i + 1} · {mb(archivo.size)} MB
                </span>
              </div>

              <button
                aria-label={`Quitar la hoja ${i + 1}`}
                className="absolute end-1.5 top-1.5 grid size-[23px] place-items-center rounded-full bg-tinta/70 text-white opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
                type="button"
                onClick={() => {
                  setArchivos(archivos.filter((_, j) => j !== i));
                  setAviso(null);
                }}
              >
                <X className="size-4" />
              </button>
            </motion.div>
          ))}
        </AnimatePresence>

        {puedeAnadir && (
          <motion.label
            className="flex aspect-[3/4] w-[104px] shrink-0 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-[#CFCBC2] bg-surface text-tenue transition-colors hover:border-tinta hover:bg-[#F6F4F0] hover:text-tinta"
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
          >
            <input
              ref={entrada}
              accept={TIPOS.join(",")}
              className="sr-only"
              multiple
              type="file"
              onChange={(e) => {
                agregar(e.target.files);
                e.target.value = ""; // permite volver a elegir el mismo archivo
              }}
            />
            <Plus className="size-6" />
            <span className="text-sm">Añadir</span>
            {archivos.length === 0 && (
              <span className="px-2 text-center text-[11px] leading-tight">
                o arrastra aquí
              </span>
            )}
          </motion.label>
        )}
      </div>

      {aviso && <Aviso tono="bloquea">{aviso}</Aviso>}

      <div className="hidden flex-col items-start gap-1.5 lg:flex">
        <Boton
          disabled={archivos.length === 0 || enviando}
          tamano="lg"
          type="submit"
        >
          {enviando ? <Girador /> : null}
          {enviando ? "Subiendo..." : "Leer mi carta"}
        </Boton>
        <p className="text-xs text-muted">
          {archivos.length === 0
            ? "Sube al menos una hoja."
            : `${archivos.length} de ${MAXIMO_HOJAS} hojas. Tarda unos diez segundos.`}
        </p>
      </div>
      <BarraAccion
        disabled={archivos.length === 0 || enviando}
        etiqueta={enviando ? "Subiendo…" : "Leer mi carta"}
        nota={
          archivos.length === 0
            ? "Sube al menos una hoja."
            : `${archivos.length} de ${MAXIMO_HOJAS} hojas. Tarda unos diez segundos.`
        }
        onClick={() => enviar()}
      />
    </form>
  );
}
