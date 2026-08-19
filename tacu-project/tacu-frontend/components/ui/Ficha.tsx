/**
 * La tarjeta del diseno: hairline de 1px y radio 16.
 *
 * `bloquea` cambia SOLO el borde, y es el unico cambio de estado que el diseno
 * pinta en el contorno: un plato que no se puede publicar se reconoce de lejos
 * sin leer nada.
 */
export function Ficha({
  bloquea,
  className = "",
  ...props
}: React.ComponentProps<"div"> & { bloquea?: boolean }) {
  return (
    <div
      className={`rounded-2xl border bg-surface ${
        bloquea ? "border-rojo-claro" : "border-border"
      } ${className}`}
      {...props}
    />
  );
}
