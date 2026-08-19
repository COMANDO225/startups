"use client";

import { motion } from "motion/react";
import type { Seccion } from "@/lib/flujo";
import type { EstadoImportacion } from "@/lib/tipos";
import { Stepper } from "./Stepper";

/**
 * Donde estas y que puedes hacer aqui. Dos formas, no una adaptada:
 *
 *  - ancho: un rail OSCURO a la izquierda, con el recorrido vertical entero
 *  - telefono: una cabecera oscura con una cinta de tres tramos y pestanas
 *
 * En el telefono el recorrido no cabe apilado —serian cinco filas antes del
 * primer plato— y plegarlo esconde justo lo que hay que ver. La cinta dice lo
 * mismo en 5 px de alto.
 *
 * El corte es en `lg` (1024) y no en `md`: el rail mide 252 px, y a 768 lo que
 * queda para la rejilla de platos no da ni para dos columnas.
 */
const estados: Record<EstadoImportacion, { texto: string; punto: string }> = {
  nueva: { texto: "Sin carta todavía", punto: "bg-white/30" },
  leyendo: { texto: "Leyendo tu carta", punto: "anima-late bg-accent" },
  lista: { texto: "Lista, sin publicar", punto: "bg-white/50" },
  publicada: { texto: "Publicada", punto: "bg-accent" },
  fallida: { texto: "La lectura falló", punto: "bg-rojo-claro" },
};

export function Lateral({
  nombre,
  estado = "nueva",
  meta,
  secciones,
  seccion,
  sub,
  onIr,
}: {
  nombre: string;
  estado?: EstadoImportacion;
  /** Lo que pasa a la derecha del titulo en el telefono: "74 platos", "en línea". */
  meta?: string;
  secciones: Seccion[];
  seccion: string;
  sub: string;
  onIr: (seccion: string, sub?: string) => void;
}) {
  const actual = secciones.find((s) => s.id === seccion);
  const est = estados[estado];

  return (
    <>
      {/* --- ancho --- */}
      <aside className="sticky top-0 hidden h-svh w-[252px] shrink-0 flex-col bg-tinta lg:flex">
        <div className="px-5 pt-5 pb-[18px]">
          <p className="font-display text-[17px] font-bold leading-none tracking-[-0.03em] text-white">
            Tacu
          </p>
          <p className="mt-1.5 truncate text-[12.5px] text-white/[0.68]">
            {nombre}
          </p>
        </div>

        <div className="flex-1 overflow-y-auto px-[14px] pb-[14px]">
          <Stepper
            onIr={onIr}
            seccion={seccion}
            secciones={secciones}
            sub={sub}
          />
        </div>

        <div className="flex items-center gap-[9px] border-t border-white/10 px-5 py-[14px]">
          <span className={`size-[7px] shrink-0 rounded-full ${est.punto}`} />
          <span className="truncate text-[11.5px] leading-[1.3] text-white/60">
            {est.texto}
          </span>
        </div>
      </aside>

      {/* --- telefono --- */}
      <div className="sticky top-0 z-30 bg-tinta lg:hidden">
        <div className="flex items-center gap-2 px-4 pt-3">
          <span className="font-display text-[15px] font-bold leading-none tracking-[-0.03em] text-white">
            Tacu
          </span>
          <span className="size-[3px] shrink-0 rounded-full bg-white/[0.28]" />
          <span className="min-w-0 flex-1 truncate text-xs leading-[1.2] text-white/50">
            {nombre}
          </span>
          <span className={`size-[7px] shrink-0 rounded-full ${est.punto}`} />
          <span className="shrink-0 text-[10.5px] font-medium leading-none text-white/50">
            {est.texto}
          </span>
        </div>

        {/* La cinta: un tramo por seccion. Alto en vez de color para marcar
            donde estas, que a 5 px se ve de reojo sin leer. */}
        <div className="flex gap-1.5 px-4 pt-[9px]">
          {secciones.map((s) => {
            const activa = s.id === seccion;
            return (
              <button
                key={s.id}
                aria-current={activa ? "step" : undefined}
                aria-label={s.titulo}
                className="flex-1 py-[7px] disabled:cursor-not-allowed"
                disabled={s.bloqueada}
                type="button"
                onClick={() => onIr(s.id)}
              >
                <span
                  className={`block w-full rounded-full transition-[height,background-color] duration-[350ms] ease-[cubic-bezier(.34,1.2,.4,1)] ${
                    activa
                      ? "h-[5px] bg-accent"
                      : s.listo
                        ? "h-[3px] bg-accent/40"
                        : "h-[3px] bg-white/[0.12]"
                  }`}
                />
              </button>
            );
          })}
        </div>

        <div className="flex items-end justify-between gap-3 px-4 pt-2.5">
          <h1 className="font-display text-[19px] font-semibold leading-[1.15] tracking-[-0.03em] text-white">
            {actual?.titulo}
          </h1>
          {meta && (
            <span className="shrink-0 pb-0.5 text-[11.5px] leading-none text-white/[0.42]">
              {meta}
            </span>
          )}
        </div>

        {actual && actual.subs.length > 0 ? (
          <div className="mt-3 px-4">
            {/* El relative va AQUI y no en el padre con padding: si no, el
                indicador mide el 100/n del ancho CON padding y se desplaza mas
                de lo que mide su pestania. */}
            <div className="relative flex">
              {actual.subs.map((sb) => {
                const aqui = sb.id === sub;
                return (
                  <button
                    key={sb.id}
                    className="flex flex-1 items-center justify-center gap-1.5 px-1 pt-[11px] pb-3"
                    type="button"
                    onClick={() => onIr(actual.id, sb.id)}
                  >
                    <span
                      className={`size-[7px] shrink-0 rounded-full transition-colors ${
                        sb.ocupado
                          ? "anima-late bg-accent"
                          : aqui
                            ? "bg-accent"
                            : sb.listo
                              ? "bg-accent/45"
                              : "bg-white/25"
                      }`}
                    />
                    <span
                      className={`truncate text-[12.5px] transition-colors ${
                        aqui ? "font-semibold text-white" : "text-white/55"
                      }`}
                    >
                      {sb.label}
                    </span>
                    {sb.tag && (
                      <span
                        className={`shrink-0 rounded-full px-1.5 py-[3px] font-display text-[9.5px] font-semibold leading-none ${
                          sb.rojo
                            ? "bg-bloquea text-white"
                            : "bg-white/[0.12] text-white/60"
                        }`}
                      >
                        {sb.tag}
                      </span>
                    )}
                  </button>
                );
              })}

              <span
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-px bg-white/[0.09]"
              />
              <motion.span
                aria-hidden
                animate={{
                  x: `${
                    Math.max(
                      0,
                      actual.subs.findIndex((sb) => sb.id === sub),
                    ) * 100
                  }%`,
                }}
                className="absolute bottom-0 left-0 h-0.5 rounded-full bg-accent"
                style={{ width: `${100 / actual.subs.length}%` }}
                transition={{ duration: 0.42, ease: [0.34, 1.2, 0.4, 1] }}
              />
            </div>
          </div>
        ) : (
          <div className="h-[14px]" />
        )}
      </div>
    </>
  );
}
