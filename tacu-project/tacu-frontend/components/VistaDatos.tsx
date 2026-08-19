"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Description, Input, Label, TextField } from "@heroui/react";
import { guardarNombre } from "@/lib/api";
import type { Importacion } from "@/lib/tipos";
import { SelectorDeTipos } from "./SelectorDeTipos";
import { Boton } from "./ui/Boton";

/**
 * Paso 1 · Tus datos: quien es el negocio.
 *
 * El tipo se elige AQUI y no en la columna del flujo. Ahi era un control suelto
 * junto a los pasos, y lo que decide —como emplata el negocio, que formatos de
 * plato existen— es un dato del restaurante, no una preferencia de la pantalla.
 */
export function VistaDatos({
  importacion,
  onSeguir,
}: {
  importacion: Importacion;
  onSeguir: () => void;
}) {
  const cliente = useQueryClient();
  const [nombre, setNombre] = useState(importacion.restaurante.nombre);

  const renombrar = useMutation({
    mutationFn: (n: string) => guardarNombre(importacion.id, n),
    onSuccess: () =>
      cliente.invalidateQueries({ queryKey: ["importacion", importacion.id] }),
  });

  // Se guarda al salir del campo y antes de continuar, nunca en cada tecla: son
  // dos llamadas en vez de una por letra.
  const guardar = () => {
    const limpio = nombre.trim();
    if (limpio && limpio !== importacion.restaurante.nombre) {
      renombrar.mutate(limpio);
    }
  };

  return (
    <div className="flex max-w-xl flex-col gap-7">
      <div>
        <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
          Tus datos
        </h2>
        <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
          Con el nombre armamos tu dirección web.
        </p>
      </div>

      <TextField
        fullWidth
        isInvalid={nombre.trim() === ""}
        value={nombre}
        onChange={setNombre}
      >
        <Label>Nombre del restaurante</Label>
        {/* onBlur va en el Input y no en el TextField: el TextField no lo
            reenvia al DOM y el nombre se perdia al salir del campo. */}
        <Input placeholder="Pollería El Rincón" onBlur={guardar} />
        {nombre.trim() === "" && (
          <Description>Sin nombre no podemos armar tu dirección.</Description>
        )}
      </TextField>

      <SelectorDeTipos idImportacion={importacion.id} />

      <div>
        <Boton
          disabled={nombre.trim() === ""}
          onClick={() => {
            guardar();
            onSeguir();
          }}
        >
          Continuar
        </Boton>
      </div>
    </div>
  );
}
