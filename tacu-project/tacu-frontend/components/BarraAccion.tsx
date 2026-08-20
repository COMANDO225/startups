"use client";

import { createPortal } from "react-dom";
import { useEsAncha } from "@/lib/pantalla";
import { Boton } from "./ui/Boton";

/**
 * La accion principal de cada paso, fija abajo. SOLO en telefono.
 *
 * Es lo que separa una web que se ve en el movil de algo que se usa como una
 * aplicacion: el dueno esta de pie en su local, con una mano, y el boton que
 * hace avanzar el flujo tiene que estar donde llega el pulgar — no al final de
 * un scroll de 74 platos.
 *
 * UNA sola accion por vista, nunca dos. Con dos, la de la derecha se pulsa sin
 * leer, y aqui las acciones cuestan dinero o publican precios.
 *
 * En pantalla ancha no existe: ahi el CTA va en linea dentro del contenido,
 * donde termina lo que acabas de hacer.
 *
 * VA POR PORTAL AL BODY, y no es un capricho: `position: fixed` deja de ser
 * fija si algun ancestro tiene un transform, y el <main> lleva la animacion de
 * entrada del panel —translateY con fill both—, que basta para convertirlo en
 * bloque contenedor. Medido: la barra acababa a 12421 px del inicio del
 * documento, o sea al final del scroll de los 74 platos.
 */
export function BarraAccion({
  etiqueta,
  nota,
  variante = "amarillo",
  disabled,
  onClick,
}: {
  etiqueta: string;
  /** Una linea centrada debajo. Para decir por que NO se puede, sobre todo. */
  nota?: string;
  variante?: "amarillo" | "tinta" | "peligro";
  disabled?: boolean;
  onClick: () => void;
}) {
  // En ancho no existe, y se decide con el hook y no con `lg:hidden`: el portal
  // necesita un body, y en el servidor no hay. El hook devuelve "ancha" ahi, asi
  // que no se monta y no hay nada que esconder.
  const ancha = useEsAncha();
  if (ancha) return null;

  return createPortal(
    <div className="anima-sube fixed inset-x-0 bottom-0 z-40 border-t border-border bg-surface/95 backdrop-blur-xl">
      {/* El padding de abajo suma la franja del indicador de inicio del
          telefono: sin el, el boton queda debajo de la barra del sistema y en
          un iPhone no se puede pulsar. */}
      <div className="px-3.5 pt-2.5 pb-[max(0.875rem,env(safe-area-inset-bottom))]">
        <Boton
          ancho
          className="min-h-[52px] rounded-[13px]"
          disabled={disabled}
          tamano="lg"
          variante={variante}
          onClick={onClick}
        >
          {etiqueta}
        </Boton>
        {nota && (
          <p className="mt-2 text-center text-[11.5px] leading-[1.35] text-tenue">
            {nota}
          </p>
        )}
      </div>
    </div>,
    document.body,
  );
}
