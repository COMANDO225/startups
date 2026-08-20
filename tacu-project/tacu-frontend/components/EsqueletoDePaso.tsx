import type { Paso } from "@/lib/flujo";
import { CatalogoEsqueleto } from "./CatalogoEsqueleto";

/**
 * El hueco de cada paso mientras llega la carta.
 *
 * Uno POR PASO y no el del catalogo para todos: ensenar ocho tarjetas de plato
 * en "Tus datos" —que no tiene platos— y cambiarlas medio segundo despues por
 * dos campos es el parpadeo. El hueco tiene que parecerse a lo que va a venir.
 */
export function EsqueletoDePaso({ paso }: { paso: Paso | null }) {
  if (paso === "revisar" || paso === "fotos") return <CatalogoEsqueleto />;

  return (
    <div className="flex flex-col gap-5">
      <Barra className="h-6 w-40" />
      <Barra className="h-4 w-72" />

      {paso === "carta" ? (
        <div className="mt-2 flex gap-2.5">
          <Barra className="aspect-[3/4] w-[104px] rounded-xl" />
          <Barra className="aspect-[3/4] w-[104px] rounded-xl" />
        </div>
      ) : paso === "publicar" ? (
        <div className="mt-2 flex flex-col gap-6 lg:flex-row">
          <Barra className="h-[470px] w-72 rounded-3xl" />
          <Barra className="h-40 flex-1 rounded-2xl" />
        </div>
      ) : (
        <div className="mt-2 flex max-w-[560px] flex-col gap-5">
          <Barra className="h-12 rounded-xl" />
          <Barra className="h-12 rounded-xl" />
        </div>
      )}
    </div>
  );
}

/** Bloque liso. Sin brillo recorriendolo: lo que se anima aqui es anadir y
 *  quitar, no esperar. */
function Barra({ className = "" }: { className?: string }) {
  return <div className={`rounded bg-surface-secondary ${className}`} />;
}
