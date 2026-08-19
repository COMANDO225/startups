"use client";

import { Check } from "lucide-react";
import { ETAPAS } from "@/lib/flujo";
import { Girador } from "./ui/Girador";

/**
 * Lo que se ve mientras la IA lee la carta.
 *
 * SIN PORCENTAJE, y es deliberado: la etapa larga —la llamada de vision— no
 * emite nada por dentro, asi que cualquier numero seria inventado. Lo que se
 * ensenia es la etapa real, que el worker anota una por una, y esa si es cierta.
 *
 * El orden de ETAPAS es el contrato: la pantalla da por hechas TODAS las
 * anteriores al indice que recibe. Si se mueve una llamada en el worker hay que
 * mover su constante y este array a la vez.
 */
export function PanelDeLectura({ etapa }: { etapa: number }) {
  return (
    <div className="anima-sube mt-[22px] rounded-[15px] border border-border bg-surface p-[17px]">
      <div className="flex items-center gap-2.5">
        <Girador />
        <p className="text-[13.5px] font-medium leading-none">
          Leyendo tu carta
        </p>
      </div>

      <ol className="mt-3">
        {ETAPAS.map((label, i) => {
          const hecha = i < etapa;
          const ahora = i === etapa;
          return (
            <li key={label} className="flex items-center gap-2.5 py-1.5">
              <span
                className={`grid size-[15px] shrink-0 place-items-center rounded-full transition-colors duration-300 ${
                  hecha
                    ? "bg-tinta text-white"
                    : ahora
                      ? "anima-late border-[1.5px] border-tinta"
                      : "border-[1.5px] border-borde-suave"
                }`}
              >
                {hecha && <Check className="size-2.5" strokeWidth={3} />}
              </span>
              <span
                className={`text-[12.5px] leading-[1.35] transition-colors ${
                  hecha || ahora ? "font-medium text-tinta" : "text-apagado"
                }`}
              >
                {label}
              </span>
            </li>
          );
        })}
      </ol>

      <p className="mt-3 border-t border-separator pt-[11px] text-xs leading-[1.5] text-tenue">
        Unos diez segundos. Todavía no hay nada que revisar.
      </p>
    </div>
  );
}
