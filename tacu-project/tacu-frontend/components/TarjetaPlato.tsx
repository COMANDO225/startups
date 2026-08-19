"use client";

import { useState } from "react";
import { Alert, Button, Card, Chip } from "@heroui/react";
import { Pencil } from "lucide-react";
import { urlMedia } from "@/lib/api";
import { useFotoDePlato } from "@/lib/hooks";
import type { Foto, Plato } from "@/lib/tipos";
import { BotonesDeFoto } from "./BotonesDeFoto";
import { PrecioEditable } from "./PrecioEditable";
import { SheetPlato } from "./SheetPlato";

/**
 * modo decide QUE se puede hacer con la tarjeta, no como se ve.
 *
 * Es la misma tarjeta en las dos vistas —el catalogo se reconoce igual— pero en
 * Datos los botones de foto sobran y estorban, y en Fotos el aviso de un precio
 * sin nombre no se puede arreglar desde ahi.
 */
export function TarjetaPlato({
  idImportacion,
  plato,
  modo = "fotos",
}: {
  idImportacion: string;
  plato: Plato;
  modo?: "datos" | "fotos";
}) {
  const [editando, setEditando] = useState(false);

  // Esta tarjeta se suscribe a SU entrada, no al mapa entero. El `select` de
  // useFotoDePlato es lo que hace la diferencia: sin el, las 74 tarjetas vuelven
  // a renderizar cada vez que UNA foto cambia.
  const { data: fotoDelPoll } = useFotoDePlato(idImportacion, plato.id);
  const foto: Foto = fotoDelPoll ?? plato.foto ?? { estado: "vacia" };

  const faltanEtiquetas =
    plato.precios.length > 1 && plato.precios.some((p) => !p.etiqueta);
  const tieneAjuste =
    !!plato.foto_ajuste || (plato.foto_referencias?.length ?? 0) > 0;

  const borde = plato.revisar
    ? plato.revisar.bloquea
      ? "border-bloquea"
      : "border-confirmar"
    : "";

  return (
    <Card
      className={`overflow-hidden ${borde}`}
      id={`plato-${plato.id}`}
      variant="secondary"
    >
      <div className="group relative">
        <RecuadroFoto foto={foto} nombre={plato.nombre} />
        <Button
          aria-label={`Editar ${plato.nombre}`}
          // Siempre visible aunque tenue: en movil no hay hover, y un control
          // que solo aparece al pasar el raton no existe para medio Peru.
          className="absolute end-2 top-2 size-8 min-w-0 rounded-full p-0 opacity-80 group-hover:opacity-100"
          size="sm"
          variant="secondary"
          onPress={() => setEditando(true)}
        >
          <Pencil className="size-3.5" />
          {tieneAjuste && (
            <span className="absolute -end-0.5 -top-0.5 size-2 rounded-full bg-warning" />
          )}
        </Button>
      </div>

      <Card.Content className="gap-3">
        {modo === "fotos" && (
          <BotonesDeFoto
            foto={foto}
            idImportacion={idImportacion}
            idPlato={plato.id}
          />
        )}

        <div>
          <h3 className="font-medium">{plato.nombre}</h3>
          {plato.descripcion && (
            <p className="mt-1 text-sm text-muted">{plato.descripcion}</p>
          )}
        </div>

        <div className="flex flex-col gap-1.5">
          {plato.precios.map((precio, i) => (
            <PrecioEditable key={i} precio={precio} />
          ))}
        </div>

        {faltanEtiquetas && (
          <Button
            className="justify-start px-0"
            size="sm"
            variant="tertiary"
            onPress={() => setEditando(true)}
          >
            Ponles nombre a los dos precios
          </Button>
        )}

        {/* Se ensena la explicacion del backend, nunca el codigo del motivo. */}
        {plato.revisar && (
          <Alert status={plato.revisar.bloquea ? "danger" : "warning"}>
            <Alert.Indicator />
            <Alert.Content>
              <Alert.Description>{plato.revisar.explicacion}</Alert.Description>
            </Alert.Content>
          </Alert>
        )}
      </Card.Content>

      <SheetPlato
        abierto={editando}
        idImportacion={idImportacion}
        plato={plato}
        onAbierto={setEditando}
      />
    </Card>
  );
}

/** Solo PINTA el estado. Las acciones son de BotonesDeFoto. */
function RecuadroFoto({ foto, nombre }: { foto: Foto; nombre: string }) {
  const caja =
    "flex h-36 w-full items-center justify-center px-4 text-center text-sm";

  switch (foto.estado) {
    case "lista":
      return (
        <div className="relative">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            alt={nombre}
            className="h-36 w-full object-cover"
            src={urlMedia(foto.url)}
          />
          {foto.origen === "propia" && (
            <Chip className="absolute start-2 top-2" size="sm">
              tu foto
            </Chip>
          )}
        </div>
      );

    case "pendiente":
      return (
        <div className={`${caja} animate-pulse bg-surface-secondary text-muted`}>
          En cola...
        </div>
      );

    case "generando":
      return (
        <div className={`${caja} animate-pulse bg-surface-secondary text-muted`}>
          Dibujando la foto...
        </div>
      );

    case "error":
      return (
        <div className={`${caja} bg-danger-soft text-danger-soft-foreground`}>
          No salio la foto
        </div>
      );

    // No es un fallo del dueno ni del sistema: se dice que pasa sin pintarlo de rojo.
    case "sin_presupuesto":
      return (
        <div className={`${caja} bg-surface-secondary text-muted`}>
          Se acabo el presupuesto de fotos de esta carta
        </div>
      );

    default:
      return (
        <div className={`${caja} bg-surface-secondary text-muted`}>
          Sin foto todavia
        </div>
      );
  }
}
