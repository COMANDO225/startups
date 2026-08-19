"use client";

/**
 * La pildora del diseno: radio 99, 12px, sin sombra.
 *
 * Sirve de chip de filtro, de etiqueta y de contador. Cuando lleva onClick sale
 * como <button>; si no, como <span>, para no meter botones en sitios donde no
 * hay nada que pulsar.
 */
const pieles = {
  on: "border-tinta bg-tinta text-white",
  off: "border-borde-suave bg-surface text-muted hover:border-tinta hover:text-tinta",
  roja: "border-bloquea bg-bloquea text-white",
  rojaOff: "border-rojo-borde bg-surface text-bloquea hover:bg-rojo-fondo",
} as const;

export function Pastilla({
  piel = "off",
  className = "",
  onClick,
  ...props
}: React.ComponentProps<"button"> & { piel?: keyof typeof pieles }) {
  const clases = `inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border px-[13px] py-[9px] text-xs font-medium leading-none transition-colors ${pieles[piel]} ${className}`;

  if (!onClick) {
    return (
      <span className={clases} {...(props as React.ComponentProps<"span">)} />
    );
  }
  return (
    <button className={clases} type="button" onClick={onClick} {...props} />
  );
}
