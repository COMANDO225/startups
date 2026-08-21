"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Panel } from "@/components/Panel";
import {
  dibujarRanura,
  guardarEstilo,
  obtenerEstilo,
  subirFotoDeRanura,
  urlMedia,
  vaciarRanura,
} from "@/lib/api";
import type { CualRanura, Estilo, Ranura } from "@/lib/tipos";
import { Boton } from "./ui/Boton";
import { Compositor } from "./ui/Compositor";
import { Girador } from "./ui/Girador";

const TIPOS = ["image/jpeg", "image/png", "image/webp"];
const MAX_BYTES = 10 * 1024 * 1024;

// El mismo tope que el ajuste de un plato, y por lo mismo: el texto del dueno se
// suma a una plantilla que ya trae camara, luz y encuadre, y uno largo compite
// con todo eso.
const MAX_TEXTO = 500;

// Una sola. La foto de la ranura viaja en TODAS las generaciones de la carta,
// asi que cada imagen de mas se paga 74 veces, y un segundo angulo de un plato
// vacio no dice nada nuevo.
const MAX_FOTOS = 1;

const RANURAS = {
  vajilla: {
    titulo: "Tu vajilla",
    ejemplo: "Ej: plato hondo de barro, bandeja de madera, bol negro mate.",
    siempre: "Plato redondo blanco",
  },
  fondo: {
    titulo: "Tu fondo",
    ejemplo: "Ej: mesa de madera oscura, mantel de colores, sobre la arena.",
    siempre: "Blanco de catálogo",
  },
} as const;

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
  const [cual, setCual] = useState<CualRanura | null>(null);
  const [texto, setTexto] = useState("");
  const [aviso, setAviso] = useState<string | null>(null);

  const clave = ["estilo", idImportacion, categoria];
  const { data: estilo } = useQuery({
    queryKey: clave,
    queryFn: () => obtenerEstilo(idImportacion, categoria),
    enabled: abierto,
  });

  const adoptar = (nuevo: Estilo) => {
    cliente.setQueryData(clave, nuevo);
    if (cual) setTexto(nuevo[cual].texto);
  };

  const textos = (nuevo: string) => ({
    vajilla: cual === "vajilla" ? nuevo : (estilo?.vajilla.texto ?? ""),
    fondo: cual === "fondo" ? nuevo : (estilo?.fondo.texto ?? ""),
  });

  const subir = useMutation({
    mutationFn: (foto: File) =>
      subirFotoDeRanura(idImportacion, categoria, cual!, foto),
    onSuccess: adoptar,
    onError: (e: Error) => setAviso(e.message),
  });

  // Guarda antes de dibujar: el backend dibuja lo que hay guardado.
  const dibujar = useMutation({
    mutationFn: async () => {
      await guardarEstilo(idImportacion, categoria, textos(texto));
      return dibujarRanura(idImportacion, categoria, cual!);
    },
    onSuccess: adoptar,
    onError: (e: Error) => setAviso(e.message),
  });

  const vaciar = useMutation({
    mutationFn: () => vaciarRanura(idImportacion, categoria, cual!),
    onSuccess: adoptar,
    onError: (e: Error) => setAviso(e.message),
  });

  const guardar = useMutation({
    mutationFn: () => guardarEstilo(idImportacion, categoria, textos(texto)),
    onSuccess: (nuevo) => {
      cliente.setQueryData(clave, nuevo);
      setCual(null);
    },
    onError: (e: Error) => setAviso(e.message),
  });

  const ocupado = subir.isPending || dibujar.isPending || vaciar.isPending;

  function abrir(r: CualRanura) {
    setTexto(estilo?.[r].texto ?? "");
    setAviso(null);
    setCual(r);
  }

  function elegir(foto: File) {
    if (!TIPOS.includes(foto.type)) {
      setAviso(`"${foto.name}" no es una foto.`);
      return;
    }
    if (foto.size > MAX_BYTES) {
      setAviso("La foto pesa más de 10 MB.");
      return;
    }
    setAviso(null);
    subir.mutate(foto);
  }

  const pie = !estilo ? undefined : cual ? (
    <>
      {estilo[cual].tocada && (
        <button
          className="me-auto text-[12.5px] font-medium text-muted underline underline-offset-[3px] disabled:text-apagado"
          disabled={ocupado}
          type="button"
          onClick={() => vaciar.mutate()}
        >
          {vaciar.isPending ? "Quitando…" : "Volver al de siempre"}
        </button>
      )}
      <Boton
        disabled={ocupado || guardar.isPending}
        onClick={() => guardar.mutate()}
      >
        {guardar.isPending ? "Guardando…" : "Listo"}
      </Boton>
    </>
  ) : (
    <Boton onClick={() => onAbierto(false)}>Listo</Boton>
  );

  return (
    <Panel
      abierto={abierto}
      atras={cual ? () => setCual(null) : undefined}
      descripcion={
        cual
          ? undefined
          : categoria
            ? `Así servimos los platos de ${categoria}.`
            : "Así servimos tus platos."
      }
      pie={pie}
      titulo={
        cual
          ? RANURAS[cual].titulo
          : categoria
            ? `Estilo de ${categoria}`
            : "Estilo de tus fotos"
      }
      onAbierto={(v) => {
        if (!v) setCual(null);
        onAbierto(v);
      }}
    >
      {!estilo ? (
        <Girador />
      ) : !cual ? (
        <div className="grid grid-cols-2 gap-6">
          {(["vajilla", "fondo"] as const).map((r) => (
            <Resumen
              key={r}
              cual={r}
              ranura={estilo[r]}
              onCambiar={() => abrir(r)}
            />
          ))}
        </div>
      ) : (
        <>
          <div className="flex justify-center">
            <Vista grande cual={cual} ranura={estilo[cual]} />
          </div>

          <div className="mt-5">
            <Compositor
              adjuntando={subir.isPending}
              ayuda={`${RANURAS[cual].ejemplo} Dibujarlo gasta una foto de tu carta.`}
              deshabilitado={ocupado}
              enviando={dibujar.isPending}
              etiquetaEnviar="Dibujarlo"
              maxAdjuntos={MAX_FOTOS}
              maximo={MAX_TEXTO}
              placeholder={`Describe ${RANURAS[cual].titulo.toLowerCase()}. Si subes una foto, nos guiamos de ella.`}
              tiposAdjunto={TIPOS}
              valor={texto}
              onAdjuntar={elegir}
              onCambio={setTexto}
              onEnviar={() => dibujar.mutate()}
            />
          </div>

          {aviso && <p className="mt-3 text-sm text-bloquea">{aviso}</p>}
        </>
      )}
    </Panel>
  );
}

function Resumen({
  cual,
  ranura,
  onCambiar,
}: {
  cual: CualRanura;
  ranura: Ranura;
  onCambiar: () => void;
}) {
  return (
    <div className="flex flex-col gap-2.5">
      <Vista cual={cual} ranura={ranura} />
      <div>
        <p className="text-sm font-medium leading-5">{RANURAS[cual].titulo}</p>
        <p className="truncate text-xs leading-4 text-tenue">
          {ranura.texto || RANURAS[cual].siempre}
        </p>
      </div>
      <Boton ancho tamano="sm" variante="blanco" onClick={onCambiar}>
        Cambiar
      </Boton>
    </div>
  );
}

/**
 * El fondo por defecto no tiene foto: es blanco liso con la sombra del plato.
 * Dibujarlo aqui evita generar —y cobrar— una imagen que ya sabemos como es.
 */
function Vista({
  cual,
  ranura,
  grande,
}: {
  cual: CualRanura;
  ranura: Ranura;
  grande?: boolean;
}) {
  const marco = `relative aspect-square shrink-0 overflow-hidden rounded-2xl border border-border ${
    grande ? "w-[196px]" : "w-full"
  }`;

  if (!ranura.imagen && cual === "fondo") {
    return (
      <div
        className={`${marco} grid place-items-center bg-linear-to-b from-white via-crema to-[#f3f1ed]`}
      >
        <div className="h-3.5 w-[62%] rounded-full bg-radial from-tinta/10 to-transparent" />
      </div>
    );
  }

  return (
    <div className={`${marco} bg-surface-secondary`}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        alt={RANURAS[cual].titulo}
        className="size-full object-cover"
        src={ranura.imagen ? urlMedia(ranura.imagen) : "/estilo/plato.jpg"}
      />
      {ranura.propia && (
        <span className="absolute start-2 top-2 rounded-full bg-white/90 px-[7px] py-1 text-[11px] font-medium leading-none text-muted">
          tuya
        </span>
      )}
    </div>
  );
}
