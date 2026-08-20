"use client";

import { AnimatePresence, motion } from "motion/react";
import { Check, Lock } from "lucide-react";
import type { Seccion } from "@/lib/flujo";

/**
 * El eje del flujo dentro del rail oscuro.
 *
 * LA CAPSULA es todo el mecanismo: una pildora amarilla que arranca en el
 * circulo de la seccion y CRECE hacia abajo hasta el sub-paso donde estas. Su
 * largo dice a que profundidad del paso estas, sin numeros ni porcentajes.
 *
 * Al cambiar de seccion los sub-pasos se pliegan y la capsula se encoge hasta
 * volver a ser el circulo: la seccion se compacto, y su indicador tambien.
 *
 * LAS MEDIDAS ESTAN ATADAS. Cada fila mide FILA y el circulo CIRCULO, asi que
 * el circulo cae a (FILA-CIRCULO)/2 del borde. De ahi salen el alto de la
 * capsula y el del carril; cambiar una obliga a recalcular las otras.
 */
const FILA = 38;
const CIRCULO = 30;
const MARGEN = (FILA - CIRCULO) / 2;

/** Hasta donde llega la capsula con el sub-paso `i` activo. */
const largoCapsula = (i: number) => FILA * i + FILA + CIRCULO;

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
      {secciones.map((s) => {
        const activa = s.id === seccion;
        const abierta = activa && s.subs.length > 0;
        const iActivo = s.subs.findIndex((sb) => sb.id === sub);
        // El numero se convierte en un visto cuando ya esta hecha Y no estamos
        // en ella: dentro de la seccion, el numero sigue diciendo donde estas.
        const hecha = s.listo && !activa;

        return (
          <div key={s.id} className="relative">
            {/* El carril: la sombra de la capsula, del largo de la seccion
                entera. Solo existe desplegada, que es cuando hay recorrido que
                ensenar. */}
            <AnimatePresence>
              {abierta && (
                <motion.span
                  animate={{ opacity: 1 }}
                  aria-hidden
                  className="absolute start-[3px] top-1 z-1 rounded-full bg-white/[0.07]"
                  exit={{ opacity: 0 }}
                  initial={{ opacity: 0 }}
                  style={{
                    width: CIRCULO,
                    height: largoCapsula(s.subs.length - 1),
                  }}
                />
              )}
            </AnimatePresence>

            {/* La capsula. Anima el alto y no un scaleY: escalar deformaria las
                dos tapas redondas, que son justo lo que la hace una pildora. */}
            <motion.span
              animate={{
                height:
                  abierta && iActivo >= 0 ? largoCapsula(iActivo) : CIRCULO,
              }}
              aria-hidden
              className={`absolute start-[3px] top-1 z-1 rounded-full ${
                activa ? "bg-accent" : s.listo ? "bg-accent/25" : "bg-white/10"
              }`}
              initial={false}
              style={{ width: CIRCULO }}
              transition={{ duration: 0.42, ease: [0.34, 1.2, 0.4, 1] }}
            />

            {/* El resalte de la fila activa, en su PROPIA capa y debajo de la
                capsula. Puesto como fondo del boton se pintaba encima y le
                lavaba el amarillo: el circulo y la capsula tienen que verse en
                su color solido, sin nada translucido por encima. */}
            {activa && (
              <span
                aria-hidden
                className="absolute inset-x-0 top-0 rounded-full bg-white/10"
                style={{ height: FILA }}
              />
            )}

            <button
              aria-current={activa ? "step" : undefined}
              className="relative z-2 flex w-full items-center gap-[11px] rounded-full px-[3px] text-start outline-offset-2 outline-accent focus-visible:outline-2 disabled:cursor-not-allowed"
              disabled={s.bloqueada}
              style={{ height: FILA }}
              type="button"
              onClick={() => onIr(s.id)}
            >
              <span
                className={`grid shrink-0 place-items-center rounded-full font-display text-[13px] font-semibold transition-colors duration-300 ${
                  activa
                    ? "text-tinta"
                    : s.listo
                      ? "text-accent"
                      : s.bloqueada
                        ? "text-white/30"
                        : "text-white/60"
                }`}
                style={{ width: CIRCULO, height: CIRCULO }}
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
              {abierta && (
                <motion.ul
                  animate={{ height: "auto", opacity: 1 }}
                  className="overflow-hidden"
                  exit={{ height: 0, opacity: 0 }}
                  initial={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
                >
                  {s.subs.map((sb, j) => {
                    const aqui = sb.id === sub;
                    // Los que quedan DENTRO de la capsula se pintan en tinta:
                    // sobre el amarillo, un punto claro no se ve.
                    const dentro = iActivo >= 0 && j <= iActivo;
                    return (
                      <li key={sb.id}>
                        <button
                          className="relative z-2 flex w-full items-center gap-[11px] ps-[3px] text-start outline-offset-2 outline-accent focus-visible:outline-2"
                          style={{ height: FILA }}
                          type="button"
                          onClick={() => onIr(s.id, sb.id)}
                        >
                          <span
                            className="grid shrink-0 place-items-center"
                            style={{ width: CIRCULO, height: CIRCULO }}
                          >
                            <span
                              className={`rounded-full transition-[background-color,width,height] duration-300 ${
                                sb.ocupado
                                  ? "anima-late bg-tinta"
                                  : aqui
                                    ? "bg-tinta"
                                    : dentro
                                      ? "bg-tinta/35"
                                      : sb.listo
                                        ? "bg-accent/45"
                                        : "bg-white/25"
                              }`}
                              style={
                                aqui
                                  ? { width: 9, height: 9 }
                                  : { width: 7, height: 7 }
                              }
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
