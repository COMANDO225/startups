"use client";

import { AnimatePresence, motion } from "motion/react";
import { Check, Lock } from "lucide-react";
import type { Seccion } from "@/lib/flujo";

/**
 * El eje del flujo dentro del rail oscuro: las secciones colgando de una linea
 * vertical, y la abierta despliega sus sub-pasos.
 *
 * NO usa un boton con variante a proposito. Un fondo por fila, siete apilados,
 * tapa la linea — que es justo lo unico que dice que esto es un recorrido y no
 * una lista de enlaces. La seccion activa si recibe fondo, y por eso se nota.
 *
 * Las medidas estan atadas entre si: la fila mide 34 px, asi que el centro del
 * circulo cae a 17, y de ahi salen el `top` del tramo y el `-bottom` que lo
 * estira hasta el centro del circulo siguiente. Cambiar la altura de la fila
 * obliga a cambiar las tres.
 */
export function Stepper({
  secciones,
  seccion,
  sub,
  onIr,
}: {
  secciones: Seccion[];
  seccion: string;
  sub: string;
  onIr: (seccion: string, sub?: string) => void;
}) {
  return (
    <nav aria-label="Pasos" className="flex flex-col gap-1">
      {secciones.map((s, i) => {
        const activa = s.id === seccion;
        const ultima = i === secciones.length - 1;
        // El numero se convierte en un visto cuando ya esta hecha Y no estamos
        // en ella: dentro de la seccion, el numero sigue diciendo donde estas.
        const hecha = s.listo && !activa;

        return (
          <div key={s.id} className="relative">
            {!ultima && (
              <span
                aria-hidden
                className={`absolute start-[16px] top-[17px] -bottom-[9px] w-px transition-colors duration-500 ${
                  s.listo ? "bg-accent/45" : "bg-white/10"
                }`}
              />
            )}

            <button
              aria-current={activa ? "step" : undefined}
              className={`relative flex h-[34px] w-full items-center gap-[11px] rounded-full px-[3px] text-start outline-offset-2 outline-accent transition-colors focus-visible:outline-2 disabled:cursor-not-allowed ${
                activa ? "bg-white/10" : ""
              }`}
              disabled={s.bloqueada}
              type="button"
              onClick={() => onIr(s.id)}
            >
              <span
                className={`grid size-[26px] shrink-0 place-items-center rounded-full font-display text-[12.5px] font-semibold transition-colors duration-300 ${
                  activa
                    ? "bg-accent text-tinta"
                    : s.listo
                      ? "bg-accent/25 text-accent"
                      : s.bloqueada
                        ? "bg-white/[0.07] text-white/30"
                        : "bg-white/10 text-white/60"
                }`}
              >
                {s.bloqueada ? (
                  <Lock className="size-3" />
                ) : hecha ? (
                  <Check className="size-3.5" />
                ) : (
                  s.num
                )}
              </span>

              <span
                className={`min-w-0 flex-1 truncate text-[13.5px] transition-colors ${
                  activa
                    ? "font-semibold text-white"
                    : s.bloqueada
                      ? "font-medium text-white/30"
                      : "font-medium text-white/60"
                }`}
              >
                {s.titulo}
              </span>
            </button>

            <AnimatePresence initial={false}>
              {activa && s.subs.length > 0 && (
                <motion.ul
                  animate={{ height: "auto", opacity: 1 }}
                  className="overflow-hidden"
                  exit={{ height: 0, opacity: 0 }}
                  initial={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
                >
                  {s.subs.map((sb) => {
                    const aqui = sb.id === sub;
                    return (
                      <li key={sb.id}>
                        <button
                          className="flex w-full items-center gap-[11px] py-[7px] ps-[3px] text-start outline-offset-2 outline-accent focus-visible:outline-2"
                          type="button"
                          onClick={() => onIr(s.id, sb.id)}
                        >
                          <span className="grid size-[26px] shrink-0 place-items-center">
                            <span
                              className={`size-[7px] rounded-full transition-[background-color,transform] duration-300 ${
                                sb.ocupado
                                  ? "anima-late bg-accent"
                                  : aqui
                                    ? "scale-[1.15] bg-accent"
                                    : sb.listo
                                      ? "bg-accent/45"
                                      : "bg-white/25"
                              }`}
                            />
                          </span>
                          <span
                            className={`min-w-0 flex-1 truncate text-[12.5px] transition-colors ${
                              aqui ? "font-medium text-white" : "text-white/55"
                            }`}
                          >
                            {sb.label}
                          </span>
                          {sb.tag && (
                            <span
                              className={`shrink-0 rounded-full px-[7px] py-[4px] font-display text-[10.5px] font-medium leading-none ${
                                sb.rojo
                                  ? "bg-bloquea text-white"
                                  : "bg-white/[0.12] text-white/60"
                              }`}
                            >
                              {sb.tag}
                            </span>
                          )}
                        </button>
                      </li>
                    );
                  })}
                </motion.ul>
              )}
            </AnimatePresence>
          </div>
        );
      })}
    </nav>
  );
}
