"use client";

import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AnimatePresence,
  Reorder,
  motion,
  useDragControls,
} from "motion/react";
import {
  AlertTriangle,
  FileText,
  Pencil,
  GripVertical,
  Loader2,
  Plus,
  X,
} from "lucide-react";
import { Panel } from "@/components/Panel";
import {
  agregarPagina,
  quitarPaginaConPlatos,
  releerCarta,
  quitarPagina,
  reordenarPaginas,
  urlMedia,
} from "@/lib/api";
import type { Importacion, Pagina, Plato } from "@/lib/tipos";
import { PanelDeLectura } from "./PanelDeLectura";
import { SubirHojas } from "./SubirHojas";
import { Aviso } from "./ui/Aviso";
import { Boton } from "./ui/Boton";
import { Girador } from "./ui/Girador";

const MAX_PAGINAS = 4;
const TIPOS = ["image/jpeg", "image/png", "image/webp", "application/pdf"];
const MAX_BYTES = 10 * 1024 * 1024;

/**
 * El sub-paso "Tu carta": las hojas de papel que entregaste.
 *
 * Anadir una hoja lee SOLO esa hoja y suma sus platos. La otra opcion —releer la
 * carta entera— borraria los platos y con ellos las fotos ya generadas, que en
 * una carta de 74 son $2.50.
 */
export function VistaCarta({
  importacion,
  conFoto,
  onReintentar,
}: {
  importacion: Importacion;
  /** Fotos ya generadas. Una relectura se las lleva por delante. */
  conFoto: number;
  onReintentar: () => void;
}) {
  // Fallida: en rojo, con las hojas que fallaron delante y CON QUE HACER. Un
  // aviso que solo dice "no pudimos" deja al dueno repitiendo la misma foto
  // mala; lo que le falta es saber que la luz o el encuadre son lo que se puede
  // arreglar, y volver a subirlas sin perder lo que ya escribio.
  // Solo lectura al volver: las hojas ya estan leidas y lo normal es venir a
  // mirarlas, no a tocarlas.
  const [editando, setEditando] = useState(false);

  if (importacion.estado === "fallida") {
    return (
      <div className="flex max-w-3xl flex-col gap-7">
        <div>
          <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
            Tu carta
          </h2>
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            Vuelve a subirla. Tu restaurante y tus datos siguen guardados.
          </p>
        </div>

        <div className="flex flex-wrap gap-3">
          {importacion.paginas.map((pagina, i) => (
            <div
              key={pagina.clave}
              className="relative h-56 w-44 shrink-0 overflow-hidden rounded-xl border-2 border-bloquea"
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                alt={`Hoja ${i + 1}`}
                className="size-full object-cover"
                src={urlMedia(pagina.url)}
              />
              <div className="pointer-events-none absolute inset-0 bg-bloquea/25" />
              <div className="absolute inset-x-0 bottom-0 flex items-center gap-1.5 bg-bloquea px-2 py-1.5 text-white">
                <AlertTriangle className="size-3.5 shrink-0" />
                <span className="text-xs font-medium">No se pudo leer</span>
              </div>
            </div>
          ))}
        </div>

        <Aviso tono="bloquea" titulo={<>No pudimos leer tu carta</>}>
          {importacion.error ?? "La IA no sacó nada en limpio."}
        </Aviso>

        <div className="flex flex-col gap-2">
          <h3 className="font-medium">Para que salga a la primera</h3>
          <ul className="flex flex-col gap-1.5 text-sm text-muted">
            <li>· La hoja entera y de frente, sin cortar los bordes.</li>
            <li>
              · Luz pareja. Sin flash directo: rebota en el plástico y tapa los
              precios.
            </li>
            <li>· Que los precios se lean sin acercar el ojo a la pantalla.</li>
            <li>
              · Si tu carta es muy larga, súbela en dos fotos en vez de una.
            </li>
          </ul>
        </div>

        <SubirHojas idImportacion={importacion.id} />

        <button
          className="self-start text-sm text-muted underline underline-offset-4"
          type="button"
          onClick={onReintentar}
        >
          Empezar de cero con otro restaurante
        </button>
      </div>
    );
  }

  const leyendo = importacion.estado === "leyendo";
  const platos = importacion.categorias.reduce(
    (n, c) => n + c.platos.length,
    0,
  );

  // Todavia sin hojas: aqui se suben TODAS de una y se manda a leer. Despues no
  // vuelve a aparecer, porque anadir una hoja a una carta ya leida es otra cosa
  // —lee solo esa hoja y suma sus platos— y de eso se encarga Paginas.
  if (importacion.estado === "nueva") {
    return (
      <div className="flex max-w-3xl flex-col gap-7">
        <div>
          <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
            Tu carta
          </h2>
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            Hasta {MAX_PAGINAS} fotos o un PDF. Derechas y con luz: tienen que
            leerse los precios.
          </p>
        </div>
        <SubirHojas idImportacion={importacion.id} />
      </div>
    );
  }

  return (
    <div className="flex max-w-3xl flex-col gap-7">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
            Tu carta
          </h2>
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            {editando
              ? `Añade, quita o reordena tus hojas. Hasta ${MAX_PAGINAS}.`
              : "Las hojas que leímos para armar tu catálogo."}
          </p>
        </div>

        {/* Fuera del modo edicion la carta es de solo lectura, y las hojas no
            traen ni la x ni el asa. Con los controles siempre puestos, volver
            aqui a mirar es un clic de distancia de quitar una hoja que ya
            costo una lectura. */}
        {!leyendo && !editando && (
          <Boton
            tamano="sm"
            variante="blanco"
            onClick={() => setEditando(true)}
          >
            <Pencil className="size-3.5" />
            Editar
          </Boton>
        )}
      </div>

      {/* Mientras lee, las hojas ya estan enormes dentro del escaner: repetirlas
          aqui en miniatura seria enseniar dos veces la misma cosa. */}
      {!leyendo && (
        <Paginas
          editando={editando}
          idImportacion={importacion.id}
          paginas={importacion.paginas}
          platos={importacion.categorias.flatMap((c) => c.platos)}
        />
      )}

      {leyendo ? (
        <PanelDeLectura etapa={importacion.etapa} />
      ) : editando ? (
        <div className="flex flex-col items-start gap-2">
          <LeerMiCarta
            conFoto={conFoto}
            idImportacion={importacion.id}
            platos={platos}
            onListo={() => setEditando(false)}
          />
          <Boton
            tamano="sm"
            variante="fantasma"
            onClick={() => setEditando(false)}
          >
            Dejarlo como está
          </Boton>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <Resumen categorias={importacion.categorias.length} platos={platos} />
        </div>
      )}
    </div>
  );
}

function Resumen({
  platos,
  categorias,
}: {
  platos: number;
  categorias: number;
}) {
  return (
    <div className="flex flex-wrap gap-7 rounded-[14px] border border-border bg-surface px-4 py-3.5">
      <Dato n={platos} que={platos === 1 ? "plato" : "platos"} />
      <Dato
        n={categorias}
        que={categorias === 1 ? "categoría" : "categorías"}
      />
    </div>
  );
}

function Dato({ n, que }: { n: number; que: string }) {
  return (
    <div>
      <p className="font-display text-2xl font-semibold tabular-nums leading-none">
        {n}
      </p>
      <p className="mt-1 text-[11.5px] text-tenue">{que}</p>
    </div>
  );
}

function Paginas({
  idImportacion,
  paginas,
  platos,
  editando,
}: {
  idImportacion: string;
  paginas: Pagina[];
  /** Todos los platos de la carta: sirven para contar los de cada hoja. */
  platos: Plato[];
  editando: boolean;
}) {
  // La hoja que el dueno acaba de pedir quitar. El aviso se abre con ella y con
  // su cuenta de platos delante, no despues.
  const [quitando, setQuitando] = useState<Pagina | null>(null);
  const cliente = useQueryClient();
  const [orden, setOrden] = useState(paginas);
  const [aviso, setAviso] = useState<string | null>(null);
  const entrada = useRef<HTMLInputElement>(null);

  // El servidor manda la lista buena, y se adopta DURANTE el render en vez de
  // en un efecto: en un efecto seria un render de mas con la lista vieja
  // pintada. Se compara por claves y no por identidad del objeto, o el poll de
  // la importacion pisaria el arrastre a medias cada dos segundos.
  const firma = paginas.map((p) => p.clave).join("|");
  const [firmaVista, setFirmaVista] = useState(firma);
  if (firma !== firmaVista) {
    setFirmaVista(firma);
    setOrden(paginas);
  }

  const refrescar = () =>
    cliente.invalidateQueries({ queryKey: ["importacion", idImportacion] });

  const anadir = useMutation({
    mutationFn: (archivo: File) => agregarPagina(idImportacion, archivo),
    onSuccess: refrescar,
    onError: (e: Error) => setAviso(e.message),
  });

  const quitar = useMutation({
    mutationFn: async ({
      clave,
      conPlatos,
    }: {
      clave: string;
      conPlatos: boolean;
    }) => {
      if (conPlatos) await quitarPaginaConPlatos(idImportacion, clave);
      else await quitarPagina(idImportacion, clave);
    },
    onSuccess: async () => {
      setQuitando(null);
      await refrescar();
    },
    onError: (e: Error) => setAviso(e.message),
  });

  const reordenar = useMutation({
    mutationFn: (claves: string[]) => reordenarPaginas(idImportacion, claves),
    onSuccess: refrescar,
    onError: (e: Error) => setAviso(e.message),
  });

  // Se valida ANTES de mandar: en un movil con datos, subir 10 MB para que el
  // backend diga que no era una imagen son varios minutos tirados.
  function elegir(archivo: File | undefined) {
    if (!archivo) return;
    if (!TIPOS.includes(archivo.type)) {
      setAviso(`"${archivo.name}" no es una foto ni un PDF.`);
      return;
    }
    if (archivo.size > MAX_BYTES) {
      setAviso("La hoja pesa más de 10 MB.");
      return;
    }
    setAviso(null);
    anadir.mutate(archivo);
  }

  const puedeAnadir = editando && orden.length < MAX_PAGINAS;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-start gap-2.5">
        <Reorder.Group
          axis="x"
          as="ul"
          className="flex flex-wrap gap-2.5"
          values={orden}
          onReorder={setOrden}
        >
          <AnimatePresence initial={false}>
            {orden.map((pagina, i) => (
              <Hoja
                key={pagina.clave}
                bloqueado={!editando}
                numero={i + 1}
                pagina={pagina}
                quitando={
                  quitar.isPending && quitar.variables?.clave === pagina.clave
                }
                onQuitar={() => setQuitando(pagina)}
                onSoltar={() => {
                  const claves = orden.map((p) => p.clave);
                  if (claves.join("|") !== firma) reordenar.mutate(claves);
                }}
              />
            ))}
          </AnimatePresence>
        </Reorder.Group>

        {puedeAnadir && (
          <motion.label
            className="flex aspect-[3/4] w-[104px] shrink-0 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-[#CFCBC2] bg-surface text-tenue transition-colors hover:border-tinta hover:bg-[#F6F4F0] hover:text-tinta"
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
          >
            <input
              ref={entrada}
              accept={TIPOS.join(",")}
              className="sr-only"
              disabled={anadir.isPending}
              type="file"
              onChange={(e) => {
                elegir(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
            {anadir.isPending ? (
              <Girador />
            ) : (
              <>
                <Plus className="size-6" />
                <span className="text-sm">Añadir</span>
              </>
            )}
          </motion.label>
        )}
      </div>

      {orden.length > 1 && editando && (
        <p className="text-xs text-muted">
          Arrástralas para ponerlas en el orden de tu carta.
        </p>
      )}

      {aviso && <Aviso tono="bloquea">{aviso}</Aviso>}

      {quitando && (
        <AvisoDeQuitarHoja
          hoja={quitando}
          numero={orden.findIndex((p) => p.clave === quitando.clave) + 1}
          ocupado={quitar.isPending}
          platos={platos.filter((p) => p.hoja === quitando.clave)}
          // Distinguir "esta hoja no aporto nada" de "no sabemos de que hoja
          // salio nada": una carta leida antes de que guardaramos el origen no
          // tiene ninguno apuntado, y decirle al dueno que la hoja no aporto
          // platos seria mentirle por omision.
          sinOrigen={platos.length > 0 && platos.every((p) => !p.hoja)}
          onCancelar={() => setQuitando(null)}
          onQuitar={(conPlatos) =>
            quitar.mutate({ clave: quitando.clave, conPlatos })
          }
        />
      )}
    </div>
  );
}

/**
 * Lo que pasa con los platos de una hoja que se quita.
 *
 * La pregunta se hace ANTES y con el numero delante, porque las dos salidas son
 * muy distintas: mantenerlos no destruye nada y borrarlos no tiene vuelta atras
 * —si alguno tenia foto, la foto se va con el—. Elegir por el dueno en
 * cualquiera de los dos sentidos es elegir mal la mitad de las veces.
 */
function AvisoDeQuitarHoja({
  hoja,
  numero,
  platos,
  sinOrigen,
  ocupado,
  onQuitar,
  onCancelar,
}: {
  hoja: Pagina;
  numero: number;
  platos: Plato[];
  /** La carta entera se leyo antes de que guardaramos de que hoja sale cada plato. */
  sinOrigen: boolean;
  ocupado: boolean;
  onQuitar: (conPlatos: boolean) => void;
  onCancelar: () => void;
}) {
  const conFoto = platos.filter((p) => p.foto?.url).length;

  return (
    <Panel
      abierto
      descripcion={
        sinOrigen ? (
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            Esta carta se leyó antes de que guardáramos de qué hoja sale cada
            plato, así que no podemos decirte cuáles salieron de ésta. Se va
            solo la imagen.
          </p>
        ) : platos.length === 0 ? (
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            Ningún plato de tu catálogo salió de esta hoja, así que solo se va
            la imagen.
          </p>
        ) : (
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            De esta hoja salieron <strong>{platos.length} productos</strong>
            {conFoto > 0 && <>, {conFoto} con foto ya generada</>}.
          </p>
        )
      }
      titulo={`Quitar la hoja ${numero}`}
      onAbierto={(v) => !v && onCancelar()}
    >
      <div className="flex flex-col gap-5">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          alt={`Hoja ${numero}`}
          className="h-44 w-36 rounded-xl border border-border object-cover"
          src={urlMedia(hoja.url)}
        />

        {platos.length > 0 && (
          <div className="flex flex-col gap-3">
            <Boton
              disabled={ocupado}
              tamano="lg"
              variante="blanco"
              onClick={() => onQuitar(false)}
            >
              Quitar solo la imagen
            </Boton>
            <p className="-mt-2 text-xs text-muted">
              Los {platos.length} productos se quedan en tu catálogo tal como
              están.
            </p>

            <Boton
              disabled={ocupado}
              tamano="lg"
              onClick={() => onQuitar(true)}
            >
              Quitar la imagen y sus {platos.length} productos
            </Boton>
            <p className="-mt-2 text-xs text-bloquea">
              Esto no se puede deshacer
              {conFoto > 0 && (
                <>
                  , y se pierden las {conFoto}{" "}
                  {conFoto === 1 ? "foto" : "fotos"} que ya pagaste
                </>
              )}
              .
            </p>
          </div>
        )}

        {platos.length === 0 && (
          <Boton disabled={ocupado} tamano="lg" onClick={() => onQuitar(false)}>
            Quitar la hoja
          </Boton>
        )}
      </div>
    </Panel>
  );
}

function Hoja({
  pagina,
  numero,
  bloqueado,
  quitando,
  onQuitar,
  onSoltar,
}: {
  pagina: Pagina;
  numero: number;
  bloqueado: boolean;
  quitando: boolean;
  onQuitar: () => void;
  onSoltar: () => void;
}) {
  // El arrastre sale del asa y no de toda la tarjeta: con toda la tarjeta
  // arrastrable, el dedo que iba a la x en un movil mueve la hoja.
  const controles = useDragControls();
  const pdf = pagina.url.toLowerCase().endsWith(".pdf");

  return (
    <Reorder.Item
      as="li"
      className="group relative aspect-[3/4] w-[104px] shrink-0 overflow-hidden rounded-xl border border-[#E7E5E0] bg-surface-secondary"
      dragControls={controles}
      dragListener={false}
      exit={{ opacity: 0, scale: 0.9 }}
      value={pagina}
      whileDrag={{
        scale: 1.05,
        zIndex: 10,
        boxShadow: "0 12px 28px rgba(0,0,0,.18)",
      }}
      onDragEnd={onSoltar}
    >
      {pdf ? (
        <div className="flex h-full flex-col items-center justify-center gap-2 text-muted">
          <FileText className="size-8" />
          <span className="text-xs font-medium">PDF</span>
        </div>
      ) : (
        // Sin next/image: la url es del backend, que en produccion sera otro
        // dominio, y esto son 4 miniaturas que no justifican configurar remotePatterns.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          alt={`Página ${numero}`}
          className="size-full object-cover"
          draggable={false}
          loading="lazy"
          src={urlMedia(pagina.url)}
        />
      )}

      <div className="pointer-events-none absolute inset-x-0 bottom-0 px-2 pb-2 [text-shadow:0_1px_3px_rgba(0,0,0,.55)]">
        <span className="text-xs font-medium text-white">Página {numero}</span>
      </div>

      {!bloqueado && (
        <>
          <button
            aria-label={`Mover la página ${numero}`}
            className="absolute start-1.5 top-1.5 grid size-[23px] cursor-grab place-items-center rounded-full bg-tinta/70 text-white opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 active:cursor-grabbing"
            type="button"
            onPointerDown={(e) => controles.start(e)}
          >
            <GripVertical className="size-4" />
          </button>

          <button
            aria-label={`Quitar la página ${numero}`}
            className="absolute end-1.5 top-1.5 grid size-[23px] place-items-center rounded-full bg-tinta/70 text-white opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 disabled:opacity-100"
            disabled={quitando}
            type="button"
            onClick={onQuitar}
          >
            {quitando ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <X className="size-4" />
            )}
          </button>
        </>
      )}
    </Reorder.Item>
  );
}

/**
 * Leer la carta otra vez, con las hojas que haya ahora.
 *
 * Es lo unico que dispara una lectura despues de la primera: anadir o quitar
 * hojas ya no lee nada por su cuenta. Asi el dueno edita todo lo que quiera y
 * paga UNA lectura cuando termina, en vez of una por hoja tocada.
 */
function LeerMiCarta({
  idImportacion,
  platos,
  conFoto,
  onListo,
}: {
  idImportacion: string;
  platos: number;
  conFoto: number;
  onListo: () => void;
}) {
  const cliente = useQueryClient();
  const [confirmando, setConfirmando] = useState(false);

  const releer = useMutation({
    mutationFn: () => releerCarta(idImportacion),
    onSuccess: async () => {
      onListo();
      await cliente.invalidateQueries({
        queryKey: ["importacion", idImportacion],
      });
    },
  });

  if (!confirmando) {
    return (
      <Boton
        tamano="lg"
        variante="amarillo"
        onClick={() => setConfirmando(true)}
      >
        Leer mi carta
      </Boton>
    );
  }

  return (
    <div className="flex max-w-lg flex-col gap-3 rounded-xl border border-border bg-surface p-4">
      <div>
        <h4 className="font-display text-[14.5px] font-semibold leading-[1.25]">
          Leer tus hojas otra vez
        </h4>
        <p className="mt-1 text-sm text-muted">
          Las leemos otra vez y actualizamos los {platos} platos con lo que diga
          tu papel: nombres, precios y secciones.
          {conFoto > 0 && (
            <>
              {" "}
              Tus <strong>{conFoto} fotos se quedan</strong>: el plato que siga
              en la carta sigue siendo el mismo plato.
            </>
          )}{" "}
          El que ya no aparezca te lo marcamos para que decidas, no lo borramos.
        </p>
      </div>

      {releer.error && (
        <p className="text-sm text-bloquea">{releer.error.message}</p>
      )}

      <div className="flex gap-2">
        <Boton
          disabled={releer.isPending}
          tamano="sm"
          onClick={() => releer.mutate()}
        >
          {releer.isPending ? "Empezando..." : "Leerla"}
        </Boton>
        <Boton
          tamano="sm"
          variante="blanco"
          onClick={() => setConfirmando(false)}
        >
          Mejor no
        </Boton>
      </div>
    </div>
  );
}
