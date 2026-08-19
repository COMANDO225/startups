"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Button,
  Description,
  Input,
  Label,
  Separator,
  Spinner,
  TextField,
} from "@heroui/react";
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
      titulo={categoria ? `Estilo de ${categoria}` : "Estilo de tus fotos"}
      onAbierto={onAbierto}
    >
      {estilo ? (
        // El key monta un formulario nuevo al cambiar de ambito: los campos
        // nacen con su valor en vez de sincronizarse en un efecto.
        <Formulario
          key={`${idImportacion}:${categoria}`}
          categoria={categoria}
          heredado={categoria !== "" ? general?.base : undefined}
          idImportacion={idImportacion}
          inicial={estilo.base}
          referencias={estilo.base.referencias}
          vista={estilo.base.vista}
          onGuardado={() => onAbierto(false)}
        />
      ) : (
        <Spinner />
      )}
    </Panel>
  );
}

function Formulario({
  idImportacion,
  categoria,
  inicial,
  heredado,
  referencias,
  vista,
  onGuardado,
}: {
  idImportacion: string;
  categoria: string;
  inicial: { recipiente: string; fondo: string };
  heredado?: { recipiente: string; fondo: string };
  referencias: string[];
  /** URL de la vista previa ya generada. Vacia = todavia no hay. */
  vista: string;
  onGuardado: () => void;
}) {
  const cliente = useQueryClient();
  const [recipiente, setRecipiente] = useState(inicial.recipiente);
  const [fondo, setFondo] = useState(inicial.fondo);

  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["estilo", idImportacion] });

  const guardar = useMutation({
    mutationFn: () =>
      guardarEstilo(idImportacion, categoria, { recipiente, fondo }),
    onSuccess: async () => {
      await refrescar();
      onGuardado();
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

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <Label>Asi se ve tu vajilla</Label>
        <Description>
          {vista
            ? "El plato de tus fotos, vacio. Cambia el texto de abajo y vuelve a dibujarlo."
            : "Por defecto servimos en esto. Escribe abajo el tuyo y dibujalo para verlo."}
        </Description>

        <div className="flex gap-2">
          {vista ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              alt="Tu vajilla"
              className="size-40 rounded-xl border border-default object-cover"
              // urlMedia le pone delante el host de la API. Las de por defecto
              // NO pasan por aqui: esas son estaticas de esta app.
              src={urlMedia(vista)}
            />
          ) : (
            // Sin vista propia van las dos de siempre: el plato individual y la
            // fuente para compartir. Son iguales para todos los restaurantes,
            // asi que viajan con la app en vez de generarse —y cobrarse— una
            // vez por cada uno.
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
          <Button
            isDisabled={dibujar.isPending}
            size="sm"
            variant="secondary"
            onPress={() => dibujar.mutate()}
          >
            {dibujar.isPending ? "Dibujando..." : "Dibujar mi vajilla"}
          </Button>
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
          placeholder={heredado?.fondo || "Fondo blanco limpio (el de siempre)"}
        />
        <Description>
          Ej: sobre una mesa de madera, en la playa, mantel de colores. El plato
          sigue siendo lo enfocado.
        </Description>
      </TextField>

      <Separator />

      <div className="flex flex-col gap-2">
        <Label>Fotos de ejemplo</Label>
        <Description>
          Sube una foto tuya y nos guiaremos de ella para el resto.
        </Description>
        <div className="flex flex-wrap gap-2">
          {referencias.map((url) => (
            <div key={url} className="relative">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                alt="Foto de ejemplo"
                className="size-20 rounded-lg border border-default object-cover"
                src={urlMedia(url)}
              />
              <Button
                aria-label="Quitar esta foto de ejemplo"
                className="absolute -end-1.5 -top-1.5 size-6 min-w-0 rounded-full p-0"
                size="sm"
                onPress={async () => {
                  await quitarReferenciaDeBase(
                    idImportacion,
                    categoria,
                    url.replace(/^.*\/media\//, ""),
                  );
                  await refrescar();
                }}
              >
                <Trash2 className="size-3" />
              </Button>
            </div>
          ))}
          {referencias.length < 2 && (
            <SubirImagen
              onArchivo={async (archivo) => {
                await subirReferenciaDeBase(idImportacion, categoria, archivo);
                await refrescar();
              }}
            />
          )}
        </div>
      </div>

      {guardar.error && (
        <p className="text-sm text-bloquea">{guardar.error.message}</p>
      )}

      <div className="flex flex-col gap-1.5">
        <Button isDisabled={guardar.isPending} onPress={() => guardar.mutate()}>
          {guardar.isPending ? "Guardando..." : "Guardar estilo"}
        </Button>
        <p className="text-xs text-muted">
          Las fotos que ya generaste no cambian. Esto se aplica a las
          siguientes, y a las que regeneres.
        </p>
      </div>
    </div>
  );
}
