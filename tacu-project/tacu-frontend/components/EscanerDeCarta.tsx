"use client";

import { motion } from "motion/react";
import { Check, FileText } from "lucide-react";
import { ETAPAS } from "@/lib/flujo";
import { urlMedia } from "@/lib/api";
import type { Pagina } from "@/lib/tipos";

/**
 * La espera, sobre SU carta.
 *
 * La barra de luz que baja es un IDIOMA, no una medida: dice "estamos leyendo
 * esto", nunca "vas por el 40%". El progreso de verdad lo lleva el texto, y ese
 * si viene del backend etapa por etapa.
 *
 * Esa separacion es deliberada. La llamada de vision se lleva casi toda la
 * espera y no emite nada por dentro, asi que una barra atada a las etapas se
 * quedaria quieta siete segundos y luego daria cinco saltos. Antes que inventar
 * un porcentaje, el barrido no promete ninguno.
 *
 * Todo se anima con transform y opacity: una sola capa en GPU, sin repintados,
 * que esto corre en el movil del dueno de pie en su restaurante.
 */
export function EscanerDeCarta({
  paginas,
  etapa,
}: {
  paginas: Pagina[];
  etapa: number;
}) {
  return (
    <div className="flex flex-col gap-7 md:flex-row md:items-start md:gap-10">
      <div className="flex flex-wrap gap-3">
        {paginas.map((pagina, i) => (
          <Hoja
            key={pagina.clave}
            numero={i + 1}
            pagina={pagina}
            retraso={i * 0.25}
          />
        ))}
      </div>

      <div className="flex flex-col gap-4">
        <div>
          <h3 className="font-medium">Leyendo tu carta</h3>
          <p className="text-sm text-muted">
            Unos diez segundos. Nada de esto lo tecleas tú.
          </p>
        </div>
        <Etapas etapa={etapa} />
      </div>
    </div>
  );
}

function Hoja({
  pagina,
  numero,
  retraso,
}: {
  pagina: Pagina;
  numero: number;
  retraso: number;
}) {
  const pdf = pagina.url.toLowerCase().endsWith(".pdf");

  return (
    <div className="relative h-56 w-44 shrink-0 overflow-hidden rounded-xl border border-border bg-surface-secondary">
      {pdf ? (
        <div className="flex h-full flex-col items-center justify-center gap-2 text-muted">
          <FileText className="size-8" />
          <span className="text-xs font-medium">PDF</span>
        </div>
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          alt={`Hoja ${numero}`}
          className="size-full object-cover"
          draggable={false}
          src={urlMedia(pagina.url)}
        />
      )}

      {/* La hoja se apaga un poco para que la luz que pasa se note. Es un div
          encima y no un filter: un filter sobre una imagen obliga a repintar en
          cada cuadro, y esto son cuatro hojas a la vez. */}
      <div className="pointer-events-none absolute inset-0 bg-slate-950/35" />

      <motion.div
        animate={{ y: ["-30%", "130%"] }}
        className="pointer-events-none absolute inset-x-0 h-1/3"
        initial={{ y: "-30%" }}
        style={{
          background:
            "linear-gradient(to bottom, transparent, color-mix(in oklab, var(--color-accent) 45%, transparent) 55%, color-mix(in oklab, var(--color-accent) 90%, white) 82%, transparent 84%)",
        }}
        transition={{
          delay: retraso,
          duration: 2.4,
          ease: "linear",
          repeat: Infinity,
        }}
      />
    </div>
  );
}

function Etapas({ etapa }: { etapa: number }) {
  return (
    <ol className="relative flex flex-col gap-2.5">
      {ETAPAS.map((label, i) => {
        const hecha = i < etapa;
        const ahora = i === etapa;
        return (
          <li key={label} className="flex items-center gap-2.5">
            <span className="relative grid size-4 shrink-0 place-items-center">
              {/* El anillo que sale del punto activo: un solo elemento que
                  escala y se apaga. Nada de box-shadow animado, que si repinta. */}
              {ahora && (
                <motion.span
                  animate={{ opacity: [0.5, 0], scale: [1, 2.2] }}
                  className="absolute inset-0 rounded-full bg-accent"
                  transition={{
                    duration: 1.6,
                    ease: "easeOut",
                    repeat: Infinity,
                  }}
                />
              )}
              <span
                className={`relative grid size-4 place-items-center rounded-full transition-all duration-300 ${
                  hecha
                    ? "bg-accent text-white"
                    : ahora
                      ? "border-2 border-accent"
                      : "border-2 border-default"
                }`}
              >
                {hecha && <Check className="size-2.5" />}
              </span>
            </span>
            <span
              className={`text-sm transition-colors ${
                ahora
                  ? "font-medium text-foreground"
                  : hecha
                    ? "text-foreground"
                    : "text-muted"
              }`}
            >
              {label}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
