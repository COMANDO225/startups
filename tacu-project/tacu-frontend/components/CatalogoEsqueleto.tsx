/**
 * Lo que se ve mientras llega la carta.
 *
 * Bloques lisos, sin shimmer: lo que se anima en este producto es anadir y
 * quitar, no esperar. Un brillo recorriendo ocho tarjetas es ruido que ademas
 * repinta sin parar en el movil del dueno.
 */
export function CatalogoEsqueleto() {
  return (
    <div className="grid grid-cols-1 gap-2.5 lg:grid-cols-[repeat(auto-fill,minmax(196px,1fr))] lg:gap-3">
      {Array.from({ length: 8 }, (_, i) => (
        <div
          key={i}
          className="flex gap-3 rounded-2xl border border-border bg-surface p-2.5 lg:flex-col lg:gap-0 lg:p-[11px]"
        >
          <div className="size-[94px] shrink-0 rounded-[11px] bg-surface-secondary lg:aspect-[4/3] lg:size-auto lg:w-full" />
          <div className="flex-1 lg:mt-[11px]">
            <div className="h-3.5 w-4/5 rounded bg-surface-secondary" />
            <div className="mt-2 h-3.5 w-1/2 rounded bg-surface-secondary" />
            <div className="mt-3 h-4 w-1/3 rounded bg-surface-secondary" />
          </div>
        </div>
      ))}
    </div>
  );
}
