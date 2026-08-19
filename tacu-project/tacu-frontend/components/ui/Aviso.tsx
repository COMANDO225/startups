/**
 * La caja de causa del diseno.
 *
 * Los DOS niveles no se pintan igual, y esa es toda la razon de que el tono sea
 * una prop y no un color suelto: en una carta real de 42 platos salieron 19
 * senialados y los 19 eran del nivel blando. Pintados como los rotos, el dueno
 * aprende a aprobar sin leer — que es exactamente cuando se cuela el precio que
 * si estaba mal.
 */
export function Aviso({
  tono = "bloquea",
  titulo,
  children,
  className = "",
}: {
  tono?: "bloquea" | "confirmar" | "neutro";
  titulo?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  const pieles = {
    bloquea: "bg-rojo-fondo text-rojo-texto",
    confirmar: "bg-hueso text-muted",
    neutro: "border border-border bg-crema text-muted",
  };
  const titulos = {
    bloquea: "text-bloquea",
    confirmar: "text-tinta",
    neutro: "text-tinta",
  };

  return (
    <div
      className={`rounded-[10px] px-[11px] py-[10px] ${pieles[tono]} ${className}`}
      role={tono === "bloquea" ? "alert" : undefined}
    >
      {titulo && (
        <p className={`text-xs font-semibold leading-[1.35] ${titulos[tono]}`}>
          {titulo}
        </p>
      )}
      {children && (
        <div className={`text-xs leading-[1.45] ${titulo ? "mt-[3px]" : ""}`}>
          {children}
        </div>
      )}
    </div>
  );
}
