import { Chip } from "@heroui/react";
import type { Precio } from "@/lib/tipos";

/** Solo pinta. La edicion vive en SheetPlato, que es quien la persiste. */
export function PrecioEditable({ precio }: { precio: Precio }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
      {precio.etiqueta && (
        <Chip size="sm" variant="secondary">
          {precio.etiqueta}
        </Chip>
      )}

      <span className="font-semibold">{precio.soles}</span>

      {/* El impreso va al lado a proposito: el dueno compara con su carta de un
          vistazo, que es la unica forma de que revise de verdad. */}
      {precio.impreso && precio.impreso !== precio.soles && (
        <span className="text-xs text-muted">
          en la carta dice {precio.impreso}
        </span>
      )}

      {precio.manuscrito && (
        <Chip size="sm" variant="tertiary">
          escrito a mano
        </Chip>
      )}
    </div>
  );
}
