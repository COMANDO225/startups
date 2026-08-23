"use client";

import { useState } from "react";
import { Pencil } from "lucide-react";
import { urlMedia } from "@/lib/api";
import { useFotoDePlato } from "@/lib/hooks";
import type { Foto, Plato } from "@/lib/tipos";
import { BotonesDeFoto } from "./BotonesDeFoto";
import { Visor } from "./ui/Visor";
import { Aviso } from "./ui/Aviso";
import { Ficha } from "./ui/Ficha";
import { PrecioEditable } from "./PrecioEditable";
import { SheetPlato } from "./SheetPlato";

/**
 * modo dice DE QUE sub-paso es la tarjeta, y con eso que lleva dentro.
 *
 * Es la misma tarjeta en las dos vistas —el catalogo se reconoce igual— pero la
 * foto entera, caja y botones, es del sub-paso de fotos: en "Lo marcado" no hay
 * nada que hacer con ella y lo que hay que mirar es el precio.
 */
export function TarjetaPlato({
  idImportacion,
  plato,
  modo = "fotos",
}: {
  idImportacion: string;
  plato: Plato;
  modo?: "marcado" | "fotos";
}) {
  const [editando, setEditando] = useState(false);
  const [mirando, setMirando] = useState(false);

  // Esta tarjeta se suscribe a SU entrada, no al mapa entero. El `select` de
  // useFotoDePlato es lo que hace la diferencia: sin el, las 74 tarjetas vuelven
  // a renderizar cada vez que UNA foto cambia.
  const { data: fotoDelPoll } = useFotoDePlato(idImportacion, plato.id);
  const foto: Foto = fotoDelPoll ?? plato.foto ?? { estado: "vacia" };

  const faltanEtiquetas =
    plato.precios.length > 1 && plato.precios.some((p) => !p.etiqueta);
  const tieneAjuste =
    !!plato.foto_ajuste || (plato.foto_referencias?.length ?? 0) > 0;

  // La foto es del sub-paso de fotos y de ningun otro. En "Lo marcado" no se
  // puede hacer nada con ella —ni generarla ni subirla— y una caja de "Sin foto
  // todavia" del alto de la tarjeta empuja fuera de la vista lo unico que hay
  // que mirar ahi, que es el precio contra la carta.
  const enFotos = modo === "fotos";

  // Sin foto la tarjeta es solo texto, y el ritmo de 8 px que iba bien pegado a
  // una imagen se lee apretado: la foto hacia de cabecera y todo lo de abajo
  // era el cuerpo. Sola, la tarjeta tiene que separar cabecera —el nombre— de
  // cuerpo —los precios— por si misma, asi que ese hueco se abre y el resto se
  // queda como esta: el enlace pertenece al grupo de los precios.
  const hueco = enFotos ? "mt-2" : "mt-3";

  return (
    <Ficha
      bloquea={plato.revisar?.bloquea}
      className={`flex ${
        enFotos
          ? `gap-3 p-2.5 lg:flex-col lg:gap-0 lg:p-[11px] ${
              foto.estado === "lista" ? "anima-pop" : ""
            }`
          : // 14 y no 11: los 11 estaban medidos contra una foto que sangra
            // hasta el borde, y contra texto quedan cortos.
            "flex-col p-3.5"
      }`}
      id={`plato-${plato.id}`}
    >
      {/* En el telefono la tarjeta es una FILA con la foto de 94 px al lado: en
          columna, cuatro tarjetas llenan la pantalla y revisar 74 platos se
          vuelve un scroll infinito. */}
      {enFotos && (
        <div className="group relative size-[94px] shrink-0 lg:aspect-[4/3] lg:size-auto lg:w-full">
          <RecuadroFoto
            foto={foto}
            nombre={plato.nombre}
            onMirar={() => setMirando(true)}
          />

          {/* Montado solo al abrir: con 74 tarjetas en pantalla, 74 <dialog>
              vacios de mas no le hacen falta a nadie. */}
          {mirando && foto.url && (
            <Visor
              imagenes={[{ url: urlMedia(foto.url)!, titulo: plato.nombre }]}
              indice={0}
              onIndice={() => setMirando(false)}
            />
          )}

          {/* Siempre visible aunque tenue: en movil no hay hover, y un control
              que solo aparece al pasar el raton no existe para medio Peru. */}
          <button
            aria-label={`Editar ${plato.nombre}`}
            className="absolute end-1.5 top-1.5 grid size-7 place-items-center rounded-[9px] bg-hueso/90 text-tinta opacity-90 transition-opacity group-hover:opacity-100 lg:size-8"
            type="button"
            onClick={() => setEditando(true)}
          >
            <Pencil className="size-3.5" />
            {tieneAjuste && (
              <span className="absolute -end-0.5 -top-0.5 size-2 rounded-full bg-confirmar" />
            )}
          </button>

          {/* El punto de "precio corregido a mano": el nivel blando NO se pinta
              como el que bloquea, y por eso es un punto y no un borde rojo. */}
          {plato.revisar && !plato.revisar.bloquea && (
            <span className="absolute end-1.5 bottom-1.5 size-[9px] rounded-full border-[1.5px] border-white bg-confirmar" />
          )}
        </div>
      )}

      <div
        className={`flex min-w-0 flex-1 flex-col ${enFotos ? "lg:mt-[11px]" : ""}`}
      >
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-[13.5px] font-medium leading-[1.35] text-pretty">
            {plato.nombre}
          </h3>

          {/* Sin foto no hay donde colgar el lapiz: pasa a la cabecera, que es
              donde queda a la misma altura que el nombre que va a editar. */}
          {!enFotos && (
            <button
              aria-label={`Editar ${plato.nombre}`}
              className="-mt-1 -me-1 grid size-7 shrink-0 place-items-center rounded-[9px] text-tenue transition-colors hover:bg-hueso hover:text-tinta"
              type="button"
              onClick={() => setEditando(true)}
            >
              <Pencil className="size-3.5" />
            </button>
          )}
        </div>
        {plato.descripcion && (
          <p className="mt-1 text-xs leading-[1.4] text-tenue">
            {plato.descripcion}
          </p>
        )}

        <div className={`${hueco} flex flex-col gap-[7px]`}>
          {plato.precios.map((precio, i) => (
            <PrecioEditable key={i} precio={precio} />
          ))}
        </div>

        {faltanEtiquetas && (
          <button
            className="mt-2 self-start text-xs font-medium text-tinta underline underline-offset-[3px]"
            type="button"
            onClick={() => setEditando(true)}
          >
            Ponles nombre a los dos precios
          </button>
        )}

        {/* Se ensenia la explicacion del backend, nunca el codigo del motivo.
            Separado del resto: el motivo por el que este plato esta marcado no
            es una linea mas de la ficha, es la razon de que estes mirandola. */}
        {plato.revisar && (
          <Aviso
            className={enFotos ? "mt-[11px]" : "mt-3.5"}
            tono={plato.revisar.bloquea ? "bloquea" : "confirmar"}
          >
            {plato.revisar.explicacion}
          </Aviso>
        )}

        {enFotos && (
          <div className="mt-[11px]">
            <BotonesDeFoto
              foto={foto}
              idImportacion={idImportacion}
              idPlato={plato.id}
            />
          </div>
        )}
      </div>

      <SheetPlato
        abierto={editando}
        idImportacion={idImportacion}
        modo={modo}
        plato={plato}
        onAbierto={setEditando}
      />
    </Ficha>
  );
}

/** Solo PINTA el estado. Las acciones son de BotonesDeFoto. */
function RecuadroFoto({
  foto,
  nombre,
  onMirar,
}: {
  foto: Foto;
  nombre: string;
  onMirar: () => void;
}) {
  const caja =
    "flex size-full items-center justify-center overflow-hidden rounded-[11px] px-2 text-center text-[11px] leading-[1.3] lg:text-xs";

  switch (foto.estado) {
    case "lista":
      return (
        <div className="relative size-full overflow-hidden rounded-[11px] border border-border">
          <button
            aria-label={`Ver la foto de ${nombre} a tamaño completo`}
            className="size-full cursor-zoom-in"
            type="button"
            onClick={onMirar}
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              alt={nombre}
              className="size-full object-cover"
              // La de 640 para la tarjeta; el visor abre la de 1280.
              src={urlMedia(foto.url_media ?? foto.url)}
            />
          </button>
          {foto.origen === "propia" && <Etiqueta>tuya</Etiqueta>}
        </div>
      );

    case "pendiente":
      return (
        <div
          className={`${caja} border border-border bg-surface-secondary text-tenue`}
        >
          En cola
        </div>
      );

    // Sin shimmer: lo que se anima es anadir y quitar, no esperar.
    case "generando":
      return (
        <div
          className={`${caja} border border-border bg-surface-secondary text-tenue`}
        >
          Generando…
        </div>
      );

    case "error":
      return (
        <div
          className={`${caja} border border-rojo-borde bg-rojo-fondo text-bloquea`}
        >
          No salió
        </div>
      );

    // No es un fallo del dueno ni del sistema: se dice que pasa sin pintarlo de rojo.
    case "sin_presupuesto":
      return (
        <div
          className={`${caja} border border-border bg-surface-secondary text-tenue`}
        >
          Sin presupuesto
        </div>
      );

    default:
      return (
        <div
          className={`${caja} border border-dashed border-borde-campo bg-surface-secondary text-tenue`}
        >
          Sin foto todavía
        </div>
      );
  }
}

function Etiqueta({ children }: { children: React.ReactNode }) {
  return (
    <span className="absolute start-1.5 top-1.5 rounded-full bg-white/90 px-[7px] py-1 text-[10px] font-medium leading-none text-muted lg:text-[11px]">
      {children}
    </span>
  );
}
