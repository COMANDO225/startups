"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Button,
  Description,
  Input,
  Label,
  Separator,
  Tabs,
  TextArea,
  TextField,
} from "@heroui/react";
import { Panel } from "@/components/Panel";
import { Trash2 } from "lucide-react";
import {
  ajustarFotoDePlato,
  editarEtiquetas,
  generarFotos,
  quitarReferenciaDePlato,
  subirReferenciaDePlato,
  urlMedia,
} from "@/lib/api";
import type { Plato } from "@/lib/tipos";
import { SubirImagen } from "@/components/SubirImagen";

const MAX_AJUSTE = 500;

export function SheetPlato({
  idImportacion,
  plato,
  abierto,
  onAbierto,
}: {
  idImportacion: string;
  plato: Plato;
  abierto: boolean;
  onAbierto: (v: boolean) => void;
}) {
  const cliente = useQueryClient();
  const refrescarCarta = () =>
    cliente.invalidateQueries({ queryKey: ["importacion", idImportacion] });
  const refrescarFotos = () =>
    cliente.invalidateQueries({ queryKey: ["fotos", idImportacion] });

  const [etiquetas, setEtiquetas] = useState(() =>
    plato.precios.map((p) => p.etiqueta ?? ""),
  );
  const [ajuste, setAjuste] = useState(plato.foto_ajuste ?? "");

  const guardarDatos = useMutation({
    mutationFn: () => editarEtiquetas(idImportacion, plato.id, etiquetas),
    onSuccess: async () => {
      await refrescarCarta();
      onAbierto(false);
    },
  });

  // Con foto ya hecha se CORRIGE: se le manda la imagen actual y solo el
  // cambio. Sin foto no hay nada que corregir, asi que se genera de cero.
  const hayFoto = (plato.foto?.estado ?? "vacia") === "lista";

  const guardarFoto = useMutation({
    mutationFn: async (regenerar: boolean) => {
      await ajustarFotoDePlato(idImportacion, plato.id, ajuste);
      if (regenerar) {
        await generarFotos(idImportacion, {
          platos: [plato.id],
          corregir: hayFoto,
        });
      }
    },
    onSuccess: async (_, regenerar) => {
      await refrescarCarta();
      if (regenerar) await refrescarFotos();
      onAbierto(false);
    },
  });

  const referencias = plato.foto_referencias ?? [];
  const variasOpciones = plato.precios.length > 1;

  return (
    <Panel
      abierto={abierto}
      descripcion={
        plato.descripcion && (
          <p className="text-sm text-muted">{plato.descripcion}</p>
        )
      }
      titulo={plato.nombre}
      onAbierto={onAbierto}
    >
      <Tabs defaultSelectedKey={plato.revisar?.bloquea ? "datos" : "foto"}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="Que editar">
            <Tabs.Tab id="datos">
              Datos
              {plato.revisar?.bloquea && (
                <span className="ms-1.5 size-1.5 rounded-full bg-bloquea" />
              )}
              <Tabs.Indicator />
            </Tabs.Tab>
            <Tabs.Tab id="foto">
              Foto
              <Tabs.Indicator />
            </Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel className="flex flex-col gap-4 pt-4" id="datos">
          {plato.revisar && (
            <Alert status={plato.revisar.bloquea ? "danger" : "warning"}>
              <Alert.Indicator />
              <Alert.Content>
                <Alert.Description>
                  {plato.revisar.explicacion}
                </Alert.Description>
              </Alert.Content>
            </Alert>
          )}

          {plato.precios.map((precio, i) => (
            <TextField
              key={i}
              isDisabled={!variasOpciones}
              value={etiquetas[i]}
              onChange={(v) =>
                setEtiquetas((p) => p.map((x, j) => (j === i ? v : x)))
              }
            >
              <Label>
                {precio.soles}
                <span className="ms-2 text-xs font-normal text-muted">
                  en la carta dice {precio.impreso}
                  {precio.manuscrito && " · escrito a mano"}
                </span>
              </Label>
              <Input maxLength={40} placeholder="De que es? Ej: Personal" />
            </TextField>
          ))}

          {!variasOpciones && (
            <p className="text-sm text-muted">
              Un solo precio: no hace falta ponerle nombre.
            </p>
          )}

          {guardarDatos.error && (
            <p className="text-sm text-bloquea">{guardarDatos.error.message}</p>
          )}

          <Button
            isDisabled={guardarDatos.isPending || !variasOpciones}
            onPress={() => guardarDatos.mutate()}
          >
            {guardarDatos.isPending ? "Guardando..." : "Guardar"}
          </Button>
        </Tabs.Panel>

        <Tabs.Panel className="flex flex-col gap-4 pt-4" id="foto">
          <div className="flex flex-col gap-2">
            <Label>Tienes una foto de ejemplo?</Label>
            <Description>
              {referencias.length > 0
                ? "Nos guiaremos de tu foto para recrear una parecida. El texto es opcional."
                : "Si subes una, la IA se guiara de ella y no hara falta describirla."}
            </Description>

            <div className="flex flex-wrap gap-2">
              {referencias.map((url) => (
                <Miniatura
                  key={url}
                  url={url}
                  onQuitar={async () => {
                    await quitarReferenciaDePlato(
                      idImportacion,
                      plato.id,
                      claveDe(url),
                    );
                    await refrescarCarta();
                  }}
                />
              ))}
              {referencias.length < 2 && (
                <SubirImagen
                  onArchivo={async (archivo) => {
                    await subirReferenciaDePlato(
                      idImportacion,
                      plato.id,
                      archivo,
                    );
                    await refrescarCarta();
                  }}
                />
              )}
            </div>
          </div>

          <Separator />

          <TextField value={ajuste} onChange={setAjuste}>
            <Label>
              {hayFoto
                ? "Que le corriges a esta foto?"
                : referencias.length > 0
                  ? "Algo mas que anadir? (opcional)"
                  : "Como quieres que se vea?"}
            </Label>
            <TextArea
              className="h-28"
              maxLength={MAX_AJUSTE}
              placeholder="Ej: el nuestro va con mas cancha y camote grueso"
            />
            <Description>
              {hayFoto
                ? "Retocamos ESTA foto: cambiamos solo lo que pidas y el resto queda igual. Vale tanto el plato (mas cancha) como la toma (que se vea recto)."
                : "Describe tu plato. La luz y el encuadre los ponemos nosotros."}
            </Description>
          </TextField>

          {guardarFoto.error && (
            <p className="text-sm text-bloquea">{guardarFoto.error.message}</p>
          )}

          <div className="flex gap-2">
            <Button
              className="flex-1"
              isDisabled={guardarFoto.isPending}
              variant="secondary"
              onPress={() => guardarFoto.mutate(false)}
            >
              Guardar
            </Button>
            <Button
              className="flex-1"
              isDisabled={guardarFoto.isPending}
              onPress={() => guardarFoto.mutate(true)}
            >
              {guardarFoto.isPending
                ? "Pidiendo..."
                : hayFoto
                  ? "Guardar y corregir"
                  : "Guardar y generar"}
            </Button>
          </div>
        </Tabs.Panel>
      </Tabs>
    </Panel>
  );
}

/** La API devuelve URLs para pintar; borrar necesita la clave, que es la cola. */
function claveDe(url: string): string {
  return url.replace(/^.*\/media\//, "");
}

function Miniatura({ url, onQuitar }: { url: string; onQuitar: () => void }) {
  return (
    <div className="relative">
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
        onPress={onQuitar}
      >
        <Trash2 className="size-3" />
      </Button>
    </div>
  );
}
