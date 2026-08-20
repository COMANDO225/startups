"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ImagePlus, Loader2, X } from "lucide-react";

const LINEAS_MAX = 6;

export type Adjunto = { url: string; alt: string };

/**
 * El campo de prompt: texto elastico, adjuntos en miniatura y un boton de envio.
 *
 * El contador solo aparece cerca del tope: verlo desde el caracter uno hace
 * escribir corto, y aqui el detalle es lo que mejora la foto.
 */
export function Compositor({
  valor,
  onCambio,
  maximo,
  placeholder,
  ayuda,
  adjuntos = [],
  maxAdjuntos = 0,
  tiposAdjunto = [],
  onAdjuntar,
  onQuitarAdjunto,
  onEnviar,
  etiquetaEnviar,
  enviando,
  adjuntando,
  deshabilitado,
}: {
  valor: string;
  onCambio: (v: string) => void;
  maximo: number;
  placeholder?: string;
  ayuda?: React.ReactNode;
  adjuntos?: Adjunto[];
  maxAdjuntos?: number;
  tiposAdjunto?: string[];
  onAdjuntar?: (archivo: File) => void;
  onQuitarAdjunto?: (url: string) => void;
  onEnviar: () => void;
  etiquetaEnviar: string;
  enviando?: boolean;
  adjuntando?: boolean;
  deshabilitado?: boolean;
}) {
  const caja = useRef<HTMLTextAreaElement>(null);
  const archivo = useRef<HTMLInputElement>(null);
  const [mirando, setMirando] = useState<Adjunto | null>(null);

  // useLayoutEffect y no useEffect: midiendo despues del pintado, la caja da un
  // salto visible en cada tecla que cambia de linea.
  useLayoutEffect(() => {
    const t = caja.current;
    if (!t) return;
    const linea = parseFloat(getComputedStyle(t).lineHeight) || 22;
    t.style.height = "auto";
    t.style.height = `${Math.min(t.scrollHeight, linea * LINEAS_MAX)}px`;
  }, [valor]);

  const restantes = maximo - valor.length;
  const cabenMas = maxAdjuntos > 0 && adjuntos.length < maxAdjuntos;
  const ocupado = !!(enviando || adjuntando || deshabilitado);

  return (
    <div className="flex flex-col gap-1.5">
      <div className="rounded-2xl border border-borde-campo bg-surface focus-within:border-tinta focus-within:outline-2 focus-within:-outline-offset-2 focus-within:outline-tinta">
        {adjuntos.length > 0 && (
          <div className="flex flex-wrap gap-2 p-2.5 pb-0">
            {adjuntos.map((a) => (
              <div key={a.url} className="relative">
                <button
                  className="block size-14 overflow-hidden rounded-xl border border-border"
                  type="button"
                  onClick={() => setMirando(a)}
                >
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    alt={a.alt}
                    className="size-full object-cover"
                    src={a.url}
                  />
                </button>
                {onQuitarAdjunto && (
                  <button
                    aria-label={`Quitar ${a.alt}`}
                    className="absolute -end-1.5 -top-1.5 grid size-5 place-items-center rounded-full bg-tinta text-white"
                    disabled={ocupado}
                    type="button"
                    onClick={() => onQuitarAdjunto(a.url)}
                  >
                    <X className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}

        <textarea
          ref={caja}
          data-foco-propio
          className="block w-full resize-none bg-transparent px-3.5 pt-3 pb-1.5 text-[15px] leading-[1.45] outline-none placeholder:text-apagado"
          maxLength={maximo}
          placeholder={placeholder}
          rows={2}
          value={valor}
          onChange={(e) => onCambio(e.target.value)}
        />

        <div className="flex items-center gap-2 px-2.5 pb-2.5">
          {maxAdjuntos > 0 && (
            <>
              <button
                aria-label="Adjuntar una foto"
                className="grid size-9 place-items-center rounded-xl text-muted transition-colors hover:bg-hueso hover:text-tinta disabled:text-apagado disabled:hover:bg-transparent"
                disabled={ocupado || !cabenMas}
                type="button"
                onClick={() => archivo.current?.click()}
              >
                {adjuntando ? (
                  <Loader2 className="size-[18px] animate-spin" />
                ) : (
                  <ImagePlus className="size-[18px]" />
                )}
              </button>
              <input
                ref={archivo}
                accept={tiposAdjunto.join(",")}
                className="sr-only"
                type="file"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  e.target.value = "";
                  if (f) onAdjuntar?.(f);
                }}
              />
            </>
          )}

          <span className="ms-auto text-xs tabular-nums text-tenue">
            {restantes <= maximo * 0.3 && restantes}
          </span>

          <button
            className="rounded-xl bg-tinta px-[18px] py-2.5 text-[12.5px] font-semibold text-white transition-[filter] hover:brightness-125 disabled:bg-hueso disabled:text-apagado"
            disabled={ocupado || valor.trim() === ""}
            type="button"
            onClick={onEnviar}
          >
            {enviando ? "…" : etiquetaEnviar}
          </button>
        </div>
      </div>

      {ayuda && <p className="px-1 text-xs leading-4 text-tenue">{ayuda}</p>}

      {mirando && <Visor adjunto={mirando} onCerrar={() => setMirando(null)} />}
    </div>
  );
}

function Visor({
  adjunto,
  onCerrar,
}: {
  adjunto: Adjunto;
  onCerrar: () => void;
}) {
  useEffect(() => {
    const alTeclear = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onCerrar();
      }
    };
    // En captura: el dialogo que hay debajo tambien escucha Escape, y sin esto
    // una sola pulsacion cierra las dos cosas.
    document.addEventListener("keydown", alTeclear, true);
    return () => document.removeEventListener("keydown", alTeclear, true);
  }, [onCerrar]);

  return createPortal(
    <div
      className="anima-aparece fixed inset-0 z-[70] grid place-items-center bg-tinta/80 p-6"
      role="presentation"
      onClick={onCerrar}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        alt={adjunto.alt}
        className="max-h-full max-w-full rounded-2xl object-contain"
        src={adjunto.url}
      />
      <button
        aria-label="Cerrar"
        className="absolute end-4 top-4 grid size-10 place-items-center rounded-full bg-white/10 text-white"
        type="button"
        onClick={onCerrar}
      >
        <X className="size-5" />
      </button>
    </div>,
    document.body,
  );
}
