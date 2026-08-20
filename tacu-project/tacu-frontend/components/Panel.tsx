"use client";

import { Drawer, Modal } from "@heroui/react";
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
  children: React.ReactNode;
}) {
  const escritorio = useEsEscritorio();

  if (escritorio) {
    return (
      <Modal.Backdrop isOpen={abierto} onOpenChange={onAbierto}>
        <Modal.Container>
          <Modal.Dialog className="w-full sm:max-w-[420px]">
            <Modal.CloseTrigger />
            <Modal.Header>
              <Modal.Heading>{titulo}</Modal.Heading>
              {descripcion}
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
            <Drawer.Heading>{titulo}</Drawer.Heading>
            {descripcion}
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
