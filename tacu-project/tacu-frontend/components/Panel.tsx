"use client";

import { Drawer, Modal } from "@heroui/react";
import { ChevronLeft } from "lucide-react";
import { useEsEscritorio } from "@/lib/pantalla";

/**
 * El mismo contenido en dos formas: modal centrado en escritorio y drawer desde
 * abajo, con manija y arrastre, en movil.
 *
 * Se monta UNO SOLO, nunca los dos escondiendose con CSS: dos dialogos abiertos
 * a la vez se pelean el foco y el lector de pantalla anuncia el que no se ve.
 */
export function Panel({
  abierto,
  onAbierto,
  titulo,
  descripcion,
  pie,
  atras,
  children,
}: {
  abierto: boolean;
  onAbierto: (v: boolean) => void;
  titulo: string;
  descripcion?: React.ReactNode;
  /**
   * La accion principal, anclada abajo y fuera de lo que rueda.
   *
   * El cuerpo es la unica zona con scroll, asi que un boton que vive dentro se
   * va con el contenido y hay que ir a buscarlo al final. Anclado, el dialogo
   * dice siempre como se sale de el, que es lo que se espera de un modal.
   */
  pie?: React.ReactNode;
  /** Con atras, el titulo lleva flecha y el panel es un segundo nivel. */
  atras?: () => void;
  children: React.ReactNode;
}) {
  const escritorio = useEsEscritorio();

  // El parrafo lo pinta el Panel y no quien lo llama. Cada uno de los tres
  // pasaba su propio <p>: dos con text-sm/text-muted —14 px y el gris de otra
  // cosa— y uno con el del diseno. El mismo hueco con tres aspectos.
  const bajada = descripcion && (
    <p className="text-[13.5px] leading-[1.5] text-parrafo">{descripcion}</p>
  );

  const encabezado = (
    <div className="flex items-center gap-1.5">
      {atras && (
        <button
          aria-label="Volver"
          className="-ms-1.5 grid size-7 shrink-0 place-items-center rounded-full text-tinta transition-colors hover:bg-hueso"
          type="button"
          onClick={atras}
        >
          <ChevronLeft className="size-[18px]" />
        </button>
      )}
      <span className="min-w-0 truncate">{titulo}</span>
    </div>
  );

  if (escritorio) {
    return (
      <Modal.Backdrop isOpen={abierto} onOpenChange={onAbierto}>
        <Modal.Container>
          <Modal.Dialog className="w-full sm:max-w-[420px]">
            <Modal.CloseTrigger />
            <Modal.Header>
              <Modal.Heading>{encabezado}</Modal.Heading>
              {bajada}
            </Modal.Header>
            <Modal.Body>{children}</Modal.Body>
            {pie && (
              <Modal.Footer className="mt-4 border-t border-separator pt-4">
                {pie}
              </Modal.Footer>
            )}
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    );
  }

  return (
    <Drawer.Backdrop isOpen={abierto} onOpenChange={onAbierto}>
      {/* placement bottom + Handle = arrastrar para cerrar, que es lo que hace
          que se sienta nativo en el telefono. */}
      <Drawer.Content placement="bottom">
        <Drawer.Dialog className="max-h-[88dvh]">
          <Drawer.Handle />
          <Drawer.CloseTrigger />
          <Drawer.Header>
            <Drawer.Heading>{encabezado}</Drawer.Heading>
            {bajada}
          </Drawer.Header>
          <Drawer.Body>{children}</Drawer.Body>
          {pie && (
            <Drawer.Footer className="mt-4 border-t border-separator pt-4">
              {pie}
            </Drawer.Footer>
          )}
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
