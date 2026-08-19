"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Key } from "@heroui/react";
import {
  Alert,
  Button,
  Description,
  Label,
  ListBox,
  Select,
} from "@heroui/react";
import {
  Drumstick,
  Fish,
  Flame,
  Pizza,
  Sandwich,
  Soup,
  Store,
  UtensilsCrossed,
  X,
} from "lucide-react";
import { catalogoDeTipos, guardarTipos, obtenerTipos } from "@/lib/api";
import type { ConteoDeTipo, Reparto } from "@/lib/tipos";

/** El icono es presentacion y vive aqui, no en el dominio. */
const iconos: Record<string, typeof Fish> = {
  cevicheria: Fish,
  polleria: Drumstick,
  chifa: Soup,
  criollo: UtensilsCrossed,
  parrilla: Flame,
  pizzeria: Pizza,
  sanguicheria: Sandwich,
  generico: Store,
};

/**
 * Los tipos de negocio. Son VARIOS porque un negocio peruano suele ser varios:
 * la cevicheria del barrio que tambien vende pollo a la brasa. Con uno solo,
 * media carta salia emplatada como no es.
 *
 * El ORDEN importa y por eso no se deja al componente: el primero es el
 * principal, el que manda en un plato que no se parece a ninguno. Al elegir uno
 * nuevo se anade al final en vez de reordenar la lista entera.
 */
export function SelectorDeTipos({
  idImportacion,
  valor,
  onCambio,
}: {
  /**
   * Sin importacion todavia —la primera pantalla, cuando el restaurante aun no
   * existe— el catalogo se pide pelado y la eleccion la guarda quien llama al
   * crear. Con importacion, este componente la guarda solo.
   */
  idImportacion?: string;
  valor?: string[];
  onCambio?: (tipos: string[]) => void;
}) {
  const cliente = useQueryClient();
  const suelto = !idImportacion;

  const { data: tipos } = useQuery({
    queryKey: ["tipos", idImportacion ?? "catalogo"],
    queryFn: async () => {
      if (idImportacion) return obtenerTipos(idImportacion);
      // Sin carta no hay reparto que calcular: son los platos los que se
      // reparten y todavia no hay ninguno.
      const { catalogo } = await catalogoDeTipos();
      return {
        elegidos: [],
        catalogo,
        reparto: { total: 0, por_tipo: [], sin_pistas: 0, faltan: [] },
      };
    },
  });

  const guardar = useMutation({
    mutationFn: (elegidos: string[]) => guardarTipos(idImportacion!, elegidos),
    // El reparto cambia con la eleccion: se pide otra vez, no se adivina.
    onSuccess: () =>
      cliente.invalidateQueries({ queryKey: ["tipos", idImportacion] }),
  });

  if (!tipos) return null;

  // Lo que se pinta es lo que se esta guardando, no lo que devolvio el servidor:
  // si no, el chip tarda una vuelta de red en aparecer.
  const elegidos = suelto
    ? (valor ?? [])
    : guardar.isPending
      ? guardar.variables
      : tipos.elegidos;

  const aplicar = (nuevos: string[]) => {
    if (suelto) onCambio?.(nuevos);
    else guardar.mutate(nuevos);
  };
  const porClave = new Map(tipos.catalogo.map((t) => [t.clave, t]));

  const cambiar = (claves: Key[]) => {
    const pedidos = claves.map(String);
    const siguen = elegidos.filter((c) => pedidos.includes(c));
    const nuevos = pedidos.filter((c) => !elegidos.includes(c));
    aplicar([...siguen, ...nuevos]);
  };

  const quitar = (clave: string) =>
    aplicar(elegidos.filter((c) => c !== clave));

  return (
    <div className="flex flex-col gap-3">
      <Select
        fullWidth
        aria-label="Tipos de negocio"
        placeholder="Elige uno o más"
        selectionMode="multiple"
        value={elegidos}
        onChange={(v) => cambiar((Array.isArray(v) ? v : []) as Key[])}
      >
        <Label>¿Qué tipo de negocio es?</Label>

        {/* h-auto: el disparador crece con los chips en vez de recortarlos.
          items-center y no items-start: con items-start el chevron se sube al
          borde de arriba y deja de estar a la altura de los chips. */}
        <Select.Trigger className="h-auto min-h-11 items-center py-1.5">
          <Select.Value>
            {({ defaultChildren, isPlaceholder }) => {
              if (isPlaceholder || elegidos.length === 0)
                return defaultChildren;

              return (
                <span className="flex flex-wrap items-center gap-1.5 py-0.5">
                  {elegidos.map((clave) => {
                    const tipo = porClave.get(clave);
                    const Icono = iconos[clave] ?? Store;
                    return (
                      <span
                        key={clave}
                        className="group/chip flex items-center gap-1.5 rounded-full bg-default py-1 ps-2 pe-1 text-sm transition-colors hover:bg-accent-soft hover:text-accent-soft-foreground"
                      >
                        <Icono className="size-3.5 shrink-0" />
                        {tipo?.nombre ?? clave}
                        {/* No es un <button>: esto vive DENTRO del boton que abre
                          la lista, y un boton dentro de otro no es HTML valido.
                          El pointerdown se corta para que quitar un chip no
                          despliegue el menu. */}
                        <span
                          aria-label={`Quitar ${tipo?.nombre ?? clave}`}
                          className="grid size-4 place-items-center rounded-full text-muted transition-colors group-hover/chip:text-accent-soft-foreground hover:bg-accent/20"
                          role="button"
                          tabIndex={-1}
                          onClick={(e) => {
                            e.stopPropagation();
                            quitar(clave);
                          }}
                          onPointerDown={(e) => {
                            e.stopPropagation();
                            e.preventDefault();
                          }}
                        >
                          <X className="size-3" />
                        </span>
                      </span>
                    );
                  })}
                </span>
              );
            }}
          </Select.Value>
          <Select.Indicator />
        </Select.Trigger>

        <Select.Popover>
          {/* Sin ancho propio: el popover ya mide lo que el disparador, y un
            w-80 dentro dejaba media caja vacia a la derecha. */}
          <ListBox selectionMode="multiple">
            {tipos.catalogo.map((t) => {
              const Icono = iconos[t.clave] ?? Store;
              return (
                // textValue es lo que lee el teclado al escribir: sin el, con
                // hijos compuestos, el item no se puede buscar.
                // El hover se queda como viene —gris—: el azul competia con la
                // marca de elegido y la lista parecia toda seleccionada. Lo que
                // el acento pinta es lo ELEGIDO: nombre e icono.
                <ListBox.Item
                  key={t.clave}
                  className="group"
                  id={t.clave}
                  textValue={t.nombre}
                >
                  <div className="flex h-8 items-start justify-center pt-1">
                    <Icono className="size-4 shrink-0 text-muted transition-colors group-data-[selected=true]:text-accent" />
                  </div>
                  <div className="flex flex-col">
                    <Label className="group-data-[selected=true]:font-medium group-data-[selected=true]:text-accent">
                      {t.nombre}
                    </Label>
                    <Description>{t.descripcion}</Description>
                  </div>
                  <ListBox.ItemIndicator />
                </ListBox.Item>
              );
            })}
          </ListBox>
        </Select.Popover>

        <Description>
          {elegidos.length === 0
            ? "Es lo que decide cómo se emplatan tus fotos."
            : "El primero manda cuando un plato no se parece a ninguno."}
        </Description>
      </Select>

      <RepartoDeLaCarta
        reparto={tipos.reparto}
        onAnadir={(clave) => aplicar([...elegidos, clave])}
      />
    </div>
  );
}

/**
 * Que le pasa a TU carta con lo que elegiste.
 *
 * Sustituye a la frase de ayuda que estaba escrita a mano en el frontend, que
 * decia lo mismo con dos platos que con doscientos y nombraba siempre a la
 * cevicheria. Estos numeros los calcula el backend con las mismas pistas que
 * deciden la foto, asi que lo que se lee aqui es lo que va a pasar al generar.
 */
function RepartoDeLaCarta({
  reparto,
  onAnadir,
}: {
  reparto: Reparto;
  onAnadir: (clave: string) => void;
}) {
  // Sin carta leida todavia no hay nada que repartir.
  if (reparto.total === 0) return null;

  const principal: ConteoDeTipo | undefined = reparto.por_tipo[0];

  return (
    <div className="flex flex-col gap-2">
      <ul className="flex flex-col gap-1.5 rounded-xl border border-border bg-surface px-4 py-3 text-sm">
        {reparto.por_tipo.map((c) => (
          <li
            key={c.clave}
            className="flex items-baseline justify-between gap-3"
          >
            <span>{c.nombre}</span>
            <span className="tabular-nums text-muted">
              {c.platos} {c.platos === 1 ? "plato" : "platos"}
            </span>
          </li>
        ))}

        {reparto.sin_pistas > 0 && (
          <li className="flex items-baseline justify-between gap-3 text-muted">
            <span>
              {principal
                ? `No se parecen a ninguno · salen como ${principal.nombre}`
                : "No llevan guarnición de la casa"}
            </span>
            <span className="tabular-nums">
              {reparto.sin_pistas}{" "}
              {reparto.sin_pistas === 1 ? "plato" : "platos"}
            </span>
          </li>
        )}
      </ul>

      {reparto.faltan.map((f) => (
        <Alert key={f.clave} status="warning">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Description>
              {f.platos} {f.platos === 1 ? "plato parece" : "platos parecen"} de{" "}
              {f.nombre} y{f.platos === 1 ? " sale" : " salen"}{" "}
              {principal
                ? `con la guarnición de ${principal.nombre}`
                : "sin guarnición de la casa"}
              .
            </Alert.Description>
          </Alert.Content>
          <Button
            size="sm"
            variant="tertiary"
            onPress={() => onAnadir(f.clave)}
          >
            Añadir {f.nombre}
          </Button>
        </Alert>
      ))}
    </div>
  );
}
