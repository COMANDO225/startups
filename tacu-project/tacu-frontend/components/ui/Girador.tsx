/**
 * El spinner del diseno: un circulo con un cuarto en tinta.
 *
 * Sustituye al Spinner y al Skeleton de HeroUI. No hay shimmer en ningun sitio:
 * lo que se anima es anadir y quitar, no esperar.
 */
export function Girador({ tam = 14 }: { tam?: number }) {
  return (
    <span
      aria-hidden
      className="anima-gira inline-block shrink-0 rounded-full border-2 border-borde-suave border-t-tinta"
      style={{ width: tam, height: tam }}
    />
  );
}
