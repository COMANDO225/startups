/** La barra de progreso del diseno: 4px, sin texto dentro. */
export function Medidor({ de, sobre }: { de: number; sobre: number }) {
  const porciento = sobre > 0 ? Math.min(100, (de / sobre) * 100) : 0;
  return (
    <div
      aria-valuemax={sobre}
      aria-valuemin={0}
      aria-valuenow={de}
      className="h-1 w-full overflow-hidden rounded-[3px] bg-border"
      role="progressbar"
    >
      <div
        className="h-full bg-tinta transition-[width] duration-500 ease-out"
        style={{ width: `${porciento}%` }}
      />
    </div>
  );
}
