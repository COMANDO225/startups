"use client";

import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { ChevronDown } from "lucide-react";
import type { Seccion } from "@/lib/flujo";
import { AccionesDeFotos } from "./AccionesDeFotos";
import { Stepper } from "./Stepper";

/**
 * La columna del flujo: quien eres, donde estas y que puedes hacer aqui.
 *
 * En el telefono la misma columna se pliega a una sola fila —el paso donde
 * estas— y se abre al tocarla. Apilada entera son cinco filas antes del primer
 * plato, que es media pantalla gastada en decir donde estas en vez de en
 * dejarte trabajar.
 */
export function Lateral({
  nombre,
  idImportacion,
  secciones,
  seccion,
  sub,
  onIr,
  onEstilo,
}: {
  nombre: string;
  idImportacion?: string;
  secciones: Seccion[];
  seccion: string;
  sub: string;
  onIr: (seccion: string, sub?: string) => void;
  onEstilo: () => void;
}) {
  const [abierto, setAbierto] = useState(false);

  const actual = secciones.find((s) => s.id === seccion);
  const subActual = actual?.subs.find((s) => s.id === sub);
  const enFotos = seccion === "carta" && sub === "fotos";

  const ir = (s: string, sb?: string) => {
    setAbierto(false);
    onIr(s, sb);
  };

  const acciones = enFotos && idImportacion && (
    <AccionesDeFotos idImportacion={idImportacion} onEstilo={onEstilo} />
  );

  return (
    <>
      <aside className="sticky top-0 hidden h-svh w-64 shrink-0 flex-col gap-7 overflow-y-auto border-e border-border bg-surface px-4 py-6 md:flex">
        <div className="px-1">
          <p className="text-lg font-semibold tracking-tight">Tacu</p>
          <p className="truncate text-sm text-muted">{nombre}</p>
        </div>

        <Stepper onIr={ir} seccion={seccion} secciones={secciones} sub={sub} />

        {acciones && <div className="mt-auto">{acciones}</div>}
      </aside>

      <div className="sticky top-0 z-30 border-b border-border bg-surface md:hidden">
        <button
          aria-expanded={abierto}
          className="flex w-full items-center gap-3 px-4 py-3 text-start"
          type="button"
          onClick={() => setAbierto((v) => !v)}
        >
          <span className="grid size-7 shrink-0 place-items-center rounded-full bg-accent text-xs font-semibold text-accent-foreground">
            {actual?.num}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-semibold">
              {actual?.titulo}
            </span>
            {subActual && (
              <span className="block truncate text-xs text-muted">
                {subActual.label}
              </span>
            )}
          </span>
          <motion.span animate={{ rotate: abierto ? 180 : 0 }}>
            <ChevronDown className="size-4 text-muted" />
          </motion.span>
        </button>

        <AnimatePresence initial={false}>
          {abierto && (
            <motion.div
              animate={{ height: "auto", opacity: 1 }}
              className="overflow-hidden"
              exit={{ height: 0, opacity: 0 }}
              initial={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
            >
              <div className="flex flex-col gap-4 px-4 pb-4">
                <Stepper
                  onIr={ir}
                  seccion={seccion}
                  secciones={secciones}
                  sub={sub}
                />
                {acciones}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </>
  );
}
