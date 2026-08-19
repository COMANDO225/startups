"use client";

import { AnimatePresence, motion } from "motion/react";
import { Chip } from "@heroui/react";
import { Check, Lock } from "lucide-react";
import type { Seccion } from "@/lib/flujo";

/**
 * El eje del flujo: las secciones colgando de una linea vertical, y la abierta
 * despliega sus sub-pasos.
 *
 * NO usa Button de HeroUI a proposito. Un boton con variante pinta un fondo por
 * fila, y siete fondos apilados tapan la linea — que es justo lo unico que dice
 * que esto es un recorrido y no una lista de enlaces.
 *
 * Las medidas estan atadas entre si: la fila mide 36 px, asi que el centro del
 * circulo cae a 18, y de ahi salen el `top` del tramo y el `-bottom` que lo
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
    <nav aria-label="Pasos" className="flex flex-col">
      {secciones.map((s, i) => {
        const activa = s.id === seccion;
        const ultima = i === secciones.length - 1;

        return (
          <div key={s.id} className="relative">
            {!ultima && (
              <span
                aria-hidden
                className={`absolute start-[13px] top-[18px] -bottom-[18px] w-0.5 rounded-full transition-colors duration-500 ${
                  s.listo ? "bg-accent" : "bg-default"
                }`}
              />
            )}

            <button
              aria-current={activa ? "step" : undefined}
              className="group relative flex h-9 w-full items-center gap-3 rounded-lg text-start outline-offset-2 outline-accent focus-visible:outline-2 disabled:cursor-not-allowed"
              disabled={s.bloqueada}
              type="button"
              onClick={() => onIr(s.id)}
            >
              <motion.span
                animate={{ scale: activa ? 1 : 0.9 }}
                className={`grid size-7 shrink-0 place-items-center rounded-full text-xs font-semibold transition-colors duration-300 ${
                  s.listo || activa
                    ? "bg-accent text-accent-foreground"
                    : "bg-default text-muted"
                } ${activa ? "ring-4 ring-accent/15" : ""}`}
                transition={{ type: "spring", stiffness: 500, damping: 30 }}
              >
                {s.bloqueada ? (
                  <Lock className="size-3" />
                ) : s.listo ? (
                  <Check className="size-4" />
                ) : (
                  s.num
                )}
              </motion.span>

              <span
                className={`min-w-0 truncate text-sm transition-colors ${
                  activa
                    ? "font-semibold text-foreground"
                    : s.bloqueada
                      ? "text-muted"
                      : "text-muted group-hover:text-foreground"
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
                          aria-current={aqui ? "page" : undefined}
                          className="group flex h-8 w-full items-center gap-3 rounded-lg text-start outline-offset-2 outline-accent focus-visible:outline-2"
                          type="button"
                          onClick={() => onIr(s.id, sb.id)}
                        >
                          {/* relative: el tramo de linea esta posicionado y sin
                              esto se pinta ENCIMA del punto, partiendolo en dos. */}
                          <span className="relative grid size-7 shrink-0 place-items-center">
                            <span
                              className={`size-2.5 rounded-full transition-colors ${
                                sb.ocupado
                                  ? "animate-pulse bg-accent"
                                  : sb.listo || aqui
                                    ? "bg-accent"
                                    : "bg-default ring-2 ring-inset ring-muted/40"
                              }`}
                            />
                          </span>

                          <span
                            className={`min-w-0 truncate text-sm transition-colors ${
                              aqui
                                ? "font-medium text-foreground"
                                : "text-muted group-hover:text-foreground"
                            }`}
                          >
                            {sb.label}
                          </span>

                          {/* Chip y no Badge: Badge es un adorno POSICIONADO
                              sobre otro elemento y suelto se va flotando a la
                              esquina. Lo dice su propia doc. */}
                          {sb.tag && (
                            <Chip
                              className="ms-auto shrink-0 tabular-nums"
                              color={sb.rojo ? "danger" : "default"}
                              size="sm"
                              variant={sb.rojo ? "primary" : "soft"}
                            >
                              {sb.tag}
                            </Chip>
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
