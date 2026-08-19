"use client";

/**
 * El boton del diseno. Sustituye al de HeroUI.
 *
 * Es un <button> nativo a proposito: lo unico que HeroUI aportaba aqui era su
 * propio aspecto, que es justo lo que hay que quitar. El teclado, el foco y el
 * estado deshabilitado ya los da el elemento.
 *
 * Las cinco variantes son las del diseno y no hay mas. El semaforo es estricto:
 * amarillo = accion, tinta = confirmacion neutra, peligro = destruye algo.
 */
const variantes = {
  amarillo:
    "bg-accent text-tinta hover:brightness-[0.97] disabled:bg-hueso disabled:text-apagado",
  tinta:
    "bg-tinta text-white hover:brightness-125 disabled:bg-hueso disabled:text-apagado",
  blanco:
    "border border-borde-campo bg-surface text-tinta hover:border-tinta disabled:text-apagado",
  fantasma:
    "bg-hueso text-tinta hover:brightness-[0.98] disabled:bg-[#F8F7F5] disabled:text-[#C2BEB5]",
  peligro:
    "border border-rojo-borde bg-surface text-bloquea hover:bg-rojo-fondo disabled:text-apagado",
} as const;

/** Los tres tamanos del diseno, con sus radios: 9 · 11 · 12. */
const tamanos = {
  sm: "gap-1.5 rounded-[9px] px-[15px] py-[11px] text-[12.5px] font-medium leading-none",
  md: "gap-2 rounded-[11px] px-[22px] py-[14px] text-[13.5px] font-semibold leading-none",
  lg: "gap-2 rounded-xl px-[22px] py-4 text-[14.5px] font-semibold leading-none",
} as const;

export function Boton({
  variante = "tinta",
  tamano = "md",
  ancho,
  className = "",
  type = "button",
  ...props
}: React.ComponentProps<"button"> & {
  variante?: keyof typeof variantes;
  tamano?: keyof typeof tamanos;
  /** Ocupa todo el ancho. En movil casi todos los primarios lo hacen. */
  ancho?: boolean;
}) {
  return (
    <button
      className={`inline-flex items-center justify-center transition-[filter,background-color,border-color,color] disabled:cursor-not-allowed ${
        variantes[variante]
      } ${tamanos[tamano]} ${ancho ? "w-full" : ""} ${className}`}
      type={type}
      {...props}
    />
  );
}
