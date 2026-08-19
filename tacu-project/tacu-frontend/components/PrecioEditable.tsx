import type { Precio } from "@/lib/tipos";

/** Solo pinta. La edicion vive en SheetPlato, que es quien la persiste. */
export function PrecioEditable({ precio }: { precio: Precio }) {
  return (
    <div>
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
        {precio.etiqueta && (
          <span className="text-[11.5px] leading-none text-tenue">
            {precio.etiqueta}
          </span>
        )}
        {/* El precio en Space Grotesk: es la unica cifra que el dueno compara
            contra su papel, y con la fuente de texto se pierde entre el nombre. */}
        <span className="font-display text-base font-semibold leading-[1.1] tracking-[-0.01em]">
          {precio.soles}
        </span>
        {precio.manuscrito && (
          <span className="text-[11px] leading-none text-tenue">
            escrito a mano
          </span>
        )}
      </div>

      {/* El impreso va debajo a proposito: el dueno compara con su carta de un
          vistazo, que es la unica forma de que revise de verdad. */}
      {precio.impreso && precio.impreso !== precio.soles && (
        <p className="mt-1 text-[11px] leading-[1.35] text-tenue">
          en la carta dice {precio.impreso}
        </p>
      )}
    </div>
  );
}
