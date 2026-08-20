"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Description, Input, Label, Separator, TextField } from "@heroui/react";
import { Panel } from "@/components/Panel";
import { Trash2 } from "lucide-react";
import {
  generarVistaDeEstilo,
  guardarEstilo,
  obtenerEstilo,
  quitarReferenciaDeBase,
  subirReferenciaDeBase,
  urlMedia,
} from "@/lib/api";
import { SubirImagen } from "@/components/SubirImagen";
import { Boton } from "./ui/Boton";
import { Girador } from "./ui/Girador";

/**
 * El estilo tiene DOS ambitos: la base general del restaurante y la de una
 * categoria. La categoria hereda campo por campo lo que no redefine, por eso los
 * campos vacios muestran de placeholder lo heredado.
 */
export function SheetEstilo({
  idImportacion,
  categoria,
  abierto,
  onAbierto,
}: {
  idImportacion: string;
  /** Vacio = la base general del restaurante. */
  categoria: string;
  abierto: boolean;
  onAbierto: (v: boolean) => void;
}) {
  const cliente = useQueryClient();

  const { data: estilo } = useQuery({
    queryKey: ["estilo", idImportacion, categoria],
    queryFn: () => obtenerEstilo(idImportacion, categoria),
    enabled: abierto,
  });

  // La general se pide cuando hay categoria, para mostrar de placeholder lo que
  // se hereda en vez de un campo vacio sin explicacion.
  const { data: general } = useQuery({
    queryKey: ["estilo", idImportacion, ""],
    queryFn: () => obtenerEstilo(idImportacion, ""),
    enabled: abierto && categoria !== "",
  });

  const [recipiente, setRecipiente] = useState("");
  const [fondo, setFondo] = useState("");

  // Los campos se siembran DURANTE el render, no en un efecto: en un efecto
  // habria un render con el formulario vacio antes de tener los valores.
  //
  // La semilla es el ambito, y solo cuenta cuando ya llego: al cambiar de
  // categoria la query se vacia, se ensena el girador, y cuando responde se
  // adopta lo suyo. Un refetch del MISMO ambito —el que dispara guardar o
  // dibujar— no vuelve a sembrar, o le borraria al dueno lo que esta
  // escribiendo.
  const semilla = estilo ? `${idImportacion}:${categoria}` : "";
  const [sembrado, setSembrado] = useState("");
  if (estilo && semilla !== sembrado) {
    setSembrado(semilla);
    setRecipiente(estilo.base.recipiente);
    setFondo(estilo.base.fondo);
  }

  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["estilo", idImportacion] });

  const guardar = useMutation({
    mutationFn: () =>
      guardarEstilo(idImportacion, categoria, { recipiente, fondo }),
    onSuccess: async () => {
      await refrescar();
      onAbierto(false);
    },
  });

  // Guarda ANTES de dibujar: el backend dibuja lo que hay guardado, asi que sin
  // esto la vista previa enseñaria la base anterior y el dueno creeria que su
  // texto no hizo nada.
  const dibujar = useMutation({
    mutationFn: async () => {
      await guardarEstilo(idImportacion, categoria, { recipiente, fondo });
      return generarVistaDeEstilo(idImportacion, categoria);
    },
    onSuccess: refrescar,
  });

  const heredado = categoria !== "" ? general?.base : undefined;

  return (
    <Panel
      abierto={abierto}
      descripcion={
        <p className="text-sm text-muted">
          {categoria
            ? "Lo que dejes en blanco se hereda del estilo general."
            : "Se aplica a todas tus fotos. Cada seccion puede tener el suyo."}
        </p>
      }
      pie={
        estilo && (
          <>
            {guardar.error && (
              <p className="flex-1 text-xs leading-[1.4] text-bloquea">
                {guardar.error.message}
              </p>
            )}
            <Boton
              disabled={guardar.isPending}
              onClick={() => guardar.mutate()}
            >
              {guardar.isPending ? "Guardando..." : "Guardar estilo"}
            </Boton>
          </>
        )
      }
      titulo={categoria ? `Estilo de ${categoria}` : "Estilo de tus fotos"}
      onAbierto={onAbierto}
    >
      {!estilo ? (
        <Girador />
      ) : (
        <div className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <Label>Asi se ve tu vajilla</Label>
            <Description>
              {estilo.base.vista
                ? "El plato de tus fotos, vacio. Cambia el texto de abajo y vuelve a dibujarlo."
                : "Por defecto servimos en esto. Escribe abajo el tuyo y dibujalo para verlo."}
            </Description>

            <div className="flex gap-2">
              {estilo.base.vista ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  alt="Tu vajilla"
                  className="size-40 rounded-xl border border-default object-cover"
                  // urlMedia le pone delante el host de la API. Las de por
                  // defecto NO pasan por aqui: esas son estaticas de esta app.
                  src={urlMedia(estilo.base.vista)}
                />
              ) : (
                // Sin vista propia van las dos de siempre: el plato individual y
                // la fuente para compartir. Son iguales para todos los
                // restaurantes, asi que viajan con la app en vez de generarse
                // —y cobrarse— una vez por cada uno.
                ["plato", "fuente"].map((cual) => (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    key={cual}
                    alt={
                      cual === "plato"
                        ? "Plato individual"
                        : "Fuente para compartir"
                    }
                    className="size-40 rounded-xl border border-default object-cover"
                    src={`/estilo/${cual}.jpg`}
                  />
                ))
              )}
            </div>

            <div className="flex items-center gap-2">
              <Boton
                disabled={dibujar.isPending}
                tamano="sm"
                variante="blanco"
                onClick={() => dibujar.mutate()}
              >
                {dibujar.isPending ? "Dibujando..." : "Dibujar mi vajilla"}
              </Boton>
              <span className="text-xs text-muted">
                Gasta una foto de tu carta.
              </span>
            </div>

            {dibujar.error && (
              <p className="text-sm text-bloquea">{dibujar.error.message}</p>
            )}
          </div>

          <Separator />

          <TextField value={recipiente} onChange={setRecipiente}>
            <Label>En que plato sirves?</Label>
            <Input
              placeholder={
                heredado?.recipiente || "Plato redondo blanco (el de siempre)"
              }
            />
            <Description>
              Ej: plato de barro, bandeja de madera, bol hondo negro.
            </Description>
          </TextField>

          <TextField value={fondo} onChange={setFondo}>
            <Label>Sobre que fondo?</Label>
            <Input
              placeholder={
                heredado?.fondo || "Fondo blanco limpio (el de siempre)"
              }
            />
            <Description>
              Ej: sobre una mesa de madera, en la playa, mantel de colores. El
              plato sigue siendo lo enfocado.
            </Description>
          </TextField>

          <Separator />

          <div className="flex flex-col gap-2">
            <Label>Fotos de ejemplo</Label>
            <Description>
              Sube una foto tuya y nos guiaremos de ella para el resto.
            </Description>
            <div className="flex flex-wrap gap-2">
              {estilo.base.referencias.map((url) => (
                <div key={url} className="relative">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    alt="Foto de ejemplo"
                    className="size-20 rounded-lg border border-default object-cover"
                    src={urlMedia(url)}
                  />
                  <Boton
                    aria-label="Quitar esta foto de ejemplo"
                    className="absolute -end-1.5 -top-1.5 size-6 min-w-0 rounded-full p-0"
                    tamano="sm"
                    onClick={async () => {
                      await quitarReferenciaDeBase(
                        idImportacion,
                        categoria,
                        url.replace(/^.*\/media\//, ""),
                      );
                      await refrescar();
                    }}
                  >
                    <Trash2 className="size-3" />
                  </Boton>
                </div>
              ))}
              {estilo.base.referencias.length < 2 && (
                <SubirImagen
                  onArchivo={async (archivo) => {
                    await subirReferenciaDeBase(
                      idImportacion,
                      categoria,
                      archivo,
                    );
                    await refrescar();
                  }}
                />
              )}
            </div>
          </div>

          <p className="text-xs text-muted">
            Las fotos que ya generaste no cambian. Esto se aplica a las
            siguientes, y a las que regeneres.
          </p>
        </div>
      )}
    </Panel>
  );
}
