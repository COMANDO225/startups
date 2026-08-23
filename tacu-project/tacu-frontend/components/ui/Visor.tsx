"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Minus, Plus, X } from "lucide-react";

export type ImagenDelVisor = { url: string; titulo?: string };

const MAX = 8;
const MIN = 1;
const PASO = 1.6;

/**
 * Ver una imagen a tamano completo.
 *
 * <dialog> nativo y no un div: la capa superior, la trampa de foco, cerrar con
 * Esc y el fondo por ::backdrop los pone el navegador. Reimplementar eso a mano
 * es como se escriben los modales que se dejan el foco detras.
 *
 * El paneo es SCROLL de verdad, no un transform arrastrado: asi hereda la
 * inercia, la rueda, el trackpad, las barras y el teclado sin escribir nada. Lo
 * unico a mano es el pinch, porque el del navegador amplia la pagina entera
 * —botonera incluida— en vez de la foto.
 */
export function Visor({
  imagenes,
  indice,
  onIndice,
}: {
  imagenes: ImagenDelVisor[];
  /** null = cerrado. */
  indice: number | null;
  onIndice: (i: number | null) => void;
}) {
  const dialogo = useRef<HTMLDialogElement>(null);
  const abierto = indice !== null;
  const actual = abierto ? imagenes[indice] : undefined;

  useEffect(() => {
    const d = dialogo.current;
    if (!d) return;
    if (abierto && !d.open) d.showModal();
    if (!abierto && d.open) d.close();
  }, [abierto]);

  const mover = useCallback(
    (paso: number) => {
      if (indice === null || imagenes.length < 2) return;
      onIndice((indice + paso + imagenes.length) % imagenes.length);
    },
    [indice, imagenes.length, onIndice],
  );

  return (
    <dialog
      ref={dialogo}
      aria-label={actual?.titulo ?? "Imagen"}
      className="m-0 h-dvh max-h-none w-screen max-w-none overflow-hidden bg-transparent p-0 backdrop:bg-tinta/92 backdrop:backdrop-blur-sm"
      onClose={() => onIndice(null)}
    >
      {actual && indice !== null && (
        // key por indice: cambiar de imagen REMONTA la lamina, y con ella se va
        // su zoom. Sin esto habria que resetearlo en un efecto —un render en
        // cascada— y la siguiente foto abriria por una esquina al azar.
        <Lamina
          key={indice}
          hayVarias={imagenes.length > 1}
          imagen={actual}
          posicion={indice + 1}
          total={imagenes.length}
          onCerrar={() => onIndice(null)}
          onMover={mover}
        />
      )}
    </dialog>
  );
}

function Lamina({
  imagen,
  posicion,
  total,
  hayVarias,
  onMover,
  onCerrar,
}: {
  imagen: ImagenDelVisor;
  posicion: number;
  total: number;
  hayVarias: boolean;
  onMover: (paso: number) => void;
  onCerrar: () => void;
}) {
  const lienzo = useRef<HTMLDivElement>(null);
  const [escala, setEscala] = useState(1);
  /** Ancho/alto de la foto. Se sabe al cargarla, no antes. */
  const [proporcion, setProporcion] = useState<number | null>(null);

  /** Amplia dejando quieto el punto que se mira. Sin esto el zoom se va al centro. */
  const ampliar = useCallback((factor: number, x?: number, y?: number) => {
    setEscala((previa) => {
      const nueva = Math.min(MAX, Math.max(MIN, previa * factor));
      if (nueva === previa) return previa;
      const c = lienzo.current;
      if (c) {
        const caja = c.getBoundingClientRect();
        const px = (x ?? caja.left + caja.width / 2) - caja.left;
        const py = (y ?? caja.top + caja.height / 2) - caja.top;
        const r = nueva / previa;
        // Tras el repintado: el area desplazable todavia no ha crecido.
        requestAnimationFrame(() => {
          c.scrollLeft = (c.scrollLeft + px) * r - px;
          c.scrollTop = (c.scrollTop + py) * r - py;
        });
      }
      return nueva;
    });
  }, []);

  useEffect(() => {
    const alPulsar = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight") onMover(1);
      else if (e.key === "ArrowLeft") onMover(-1);
      else if (e.key === "+" || e.key === "=") ampliar(PASO);
      else if (e.key === "-") ampliar(1 / PASO);
      else if (e.key === "0") setEscala(1);
      else return;
      e.preventDefault();
    };
    window.addEventListener("keydown", alPulsar);
    return () => window.removeEventListener("keydown", alPulsar);
  }, [onMover, ampliar]);

  // El pinch, a mano con dos punteros. Y la rueda con ctrl/cmd, que es lo que
  // manda un trackpad al pellizcar; la rueda sola se queda con el scroll nativo.
  useEffect(() => {
    const c = lienzo.current;
    if (!c) return;

    const dedos = new Map<number, { x: number; y: number }>();
    let separacion = 0;
    const distancia = () => {
      const [a, b] = [...dedos.values()];
      return Math.hypot(a.x - b.x, a.y - b.y);
    };

    const abajo = (e: PointerEvent) => {
      if (e.pointerType !== "touch") return;
      dedos.set(e.pointerId, { x: e.clientX, y: e.clientY });
      if (dedos.size === 2) separacion = distancia();
    };
    const mueve = (e: PointerEvent) => {
      if (!dedos.has(e.pointerId)) return;
      dedos.set(e.pointerId, { x: e.clientX, y: e.clientY });
      if (dedos.size !== 2) return;
      e.preventDefault();
      const ahora = distancia();
      if (separacion > 0) {
        const [a, b] = [...dedos.values()];
        ampliar(ahora / separacion, (a.x + b.x) / 2, (a.y + b.y) / 2);
      }
      separacion = ahora;
    };
    const suelta = (e: PointerEvent) => {
      dedos.delete(e.pointerId);
      if (dedos.size < 2) separacion = 0;
    };
    const rueda = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      ampliar(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX, e.clientY);
    };

    c.addEventListener("pointerdown", abajo);
    c.addEventListener("pointermove", mueve, { passive: false });
    c.addEventListener("pointerup", suelta);
    c.addEventListener("pointercancel", suelta);
    c.addEventListener("wheel", rueda, { passive: false });
    return () => {
      c.removeEventListener("pointerdown", abajo);
      c.removeEventListener("pointermove", mueve);
      c.removeEventListener("pointerup", suelta);
      c.removeEventListener("pointercancel", suelta);
      c.removeEventListener("wheel", rueda);
    };
  }, [ampliar]);

  // La caja mide EXACTAMENTE la foto ajustada por la escala, en CSS puro: cqw y
  // cqh son el ancho y el alto del lienzo, asi que min(100cqw, 100cqh * p) es el
  // ancho con el que la foto encaja. Sin esto la caja crecia en los dos ejes por
  // igual y una hoja vertical dejaba franjas vacias enormes donde perderse
  // paneando. Y sin leer un solo tamano desde JS.
  const caja =
    proporcion === null
      ? { width: "100%", height: "100%" }
      : {
          width: `calc(${escala} * min(100cqw, 100cqh * ${proporcion}))`,
          height: `calc(${escala} * min(100cqh, 100cqw / ${proporcion}))`,
        };

  return (
    <div className="flex h-full flex-col">
      <div
        ref={lienzo}
        // container-type:size da las unidades cq* de abajo. `safe center` centra
        // la foto pequena sin recortarle el principio cuando crece y desborda.
        className="grid min-h-0 flex-1 overflow-auto overscroll-contain p-3 [container-type:size] [place-content:safe_center] [scrollbar-width:thin]"
        style={{ touchAction: escala > 1 ? "pan-x pan-y" : "none" }}
      >
        {/* El zoom NO se mide en JS: la caja crece con la escala y object-contain
            encaja la foto dentro. Medir el contenedor obligaria a leer el ref
            durante el render, que es justo lo que rompe cuando cambia el tamano. */}
        <div
          style={caja}
          onClick={(e) => e.target === e.currentTarget && onCerrar()}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            alt={imagen.titulo ?? "Imagen a tamano completo"}
            // Llena la caja y NO max-w/max-h: con los max la foto solo puede
            // encoger, nunca pasar de su tamano natural, y el zoom se agotaba a
            // 256% justo donde hace falta para leer un precio.
            className="size-full object-contain select-none"
            draggable={false}
            src={imagen.url}
            onLoad={(e) =>
              setProporcion(
                e.currentTarget.naturalWidth / e.currentTarget.naturalHeight,
              )
            }
            onDoubleClick={(e) =>
              escala > 1 ? setEscala(1) : ampliar(2.5, e.clientX, e.clientY)
            }
          />
        </div>
      </div>

      <div className="flex items-center justify-between gap-3 px-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] pt-3 text-white">
        <div className="flex min-w-0 items-center gap-2">
          {hayVarias && (
            <>
              <Redondo etiqueta="Anterior" onClick={() => onMover(-1)}>
                <ChevronLeft className="size-5" />
              </Redondo>
              <Redondo etiqueta="Siguiente" onClick={() => onMover(1)}>
                <ChevronRight className="size-5" />
              </Redondo>
              <span className="ms-1 shrink-0 font-display text-[12.5px] tabular-nums text-white/70">
                {posicion} / {total}
              </span>
            </>
          )}
          {imagen.titulo && (
            <span className="ms-1 min-w-0 truncate text-[12.5px] text-white/70">
              {imagen.titulo}
            </span>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <Redondo
            desactivado={escala <= MIN}
            etiqueta="Alejar"
            onClick={() => ampliar(1 / PASO)}
          >
            <Minus className="size-5" />
          </Redondo>
          <span className="w-11 text-center font-display text-[12.5px] tabular-nums text-white/70">
            {Math.round(escala * 100)}%
          </span>
          <Redondo
            desactivado={escala >= MAX}
            etiqueta="Ampliar"
            onClick={() => ampliar(PASO)}
          >
            <Plus className="size-5" />
          </Redondo>
          <Redondo etiqueta="Cerrar" onClick={onCerrar}>
            <X className="size-5" />
          </Redondo>
        </div>
      </div>
    </div>
  );
}

function Redondo({
  children,
  etiqueta,
  desactivado,
  onClick,
}: {
  children: React.ReactNode;
  etiqueta: string;
  desactivado?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      aria-label={etiqueta}
      className="grid size-10 shrink-0 place-items-center rounded-full bg-white/12 text-white transition-colors hover:bg-white/22 disabled:opacity-35"
      disabled={desactivado}
      type="button"
      onClick={onClick}
    >
      {children}
    </button>
  );
}
