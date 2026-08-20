"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { AccionesDeFotos } from "@/components/AccionesDeFotos";
import { PlatosAusentes } from "@/components/PlatosAusentes";
import { SheetEstilo } from "@/components/SheetEstilo";
import { TarjetaPlato } from "@/components/TarjetaPlato";
import { VistaCarta } from "@/components/VistaCarta";
import { VistaDatos } from "@/components/VistaDatos";
import { VistaPublicar } from "@/components/VistaPublicar";
import { Boton } from "@/components/ui/Boton";
import { pasoValido } from "@/lib/flujo";
import { useFotos, useImportacion } from "@/lib/hooks";
import type { Plato } from "@/lib/tipos";
import { BarraAccion } from "@/components/BarraAccion";

/**
 * El paso que pide la URL.
 *
 * No comprueba si se puede estar aqui: de eso se encarga el guard del layout,
 * que es el unico sitio donde vive esa decision. Esta pagina solo pinta.
 *
 * Vuelve a pedir la importacion y las fotos, y no cuesta una segunda llamada:
 * react-query las tiene cacheadas por el layout, que ya las pidio.
 */
export default function PasoDelEditor() {
  const { id, paso } = useParams<{ id: string; paso: string }>();
  const router = useRouter();
  const { data: importacion } = useImportacion(id);
  const { data: fotos } = useFotos(id);
  const [estilando, setEstilando] = useState<string | null>(null);

  if (!importacion) return null;

  const cual = pasoValido(paso);
  const lista = Object.values(fotos?.fotos ?? {});
  const conFoto = lista.filter((f) => f.estado === "lista").length;
  const platos = importacion.categorias.flatMap((c) => c.platos);
  const marcados = platos.filter((p) => p.revisar);

  return (
    <>
      {cual === "datos" && (
        <VistaDatos
          importacion={importacion}
          onSeguir={() => router.push(`/i/${id}/carta`)}
        />
      )}

      {cual === "carta" && (
        <VistaCarta
          conFoto={conFoto}
          importacion={importacion}
          onReintentar={() => router.push("/")}
          onSeguir={() => router.push(`/i/${id}/revisar`)}
        />
      )}

      {cual === "revisar" && (
        <>
          <Revisar
            idImportacion={id}
            marcados={marcados}
            onSeguir={() => router.push(`/i/${id}/fotos`)}
          />
          <BarraAccion
            etiqueta="Seguir a las fotos"
            nota={
              importacion.marcas.revisar > 0
                ? `${importacion.marcas.revisar} no se publican así.`
                : undefined
            }
            variante={importacion.marcas.revisar > 0 ? "peligro" : "tinta"}
            onClick={() => router.push(`/i/${id}/fotos`)}
          />
        </>
      )}

      {cual === "fotos" && (
        <>
          <Catalogo
            idImportacion={id}
            importacion={importacion}
            onEstiloDe={setEstilando}
          />

          {/* En ancho el CTA cierra el contenido; en telefono vive abajo. */}
          <div className="mt-8 hidden lg:block">
            <Boton
              ancho
              tamano="lg"
              variante={importacion.puede_publicarse ? "amarillo" : "blanco"}
              onClick={() => router.push(`/i/${id}/publicar`)}
            >
              Seguir a publicar
              <ArrowRight className="size-4" />
            </Boton>
            {!importacion.puede_publicarse && (
              <p className="mt-2 text-center text-[11.5px] text-tenue">
                Te faltan {importacion.marcas.revisar} platos por corregir, pero
                puedes ir viendo cómo queda.
              </p>
            )}
          </div>

          <BarraAccion
            etiqueta="Seguir a publicar"
            nota={
              importacion.puede_publicarse
                ? undefined
                : `Te faltan ${importacion.marcas.revisar} por corregir.`
            }
            variante={importacion.puede_publicarse ? "amarillo" : "tinta"}
            onClick={() => router.push(`/i/${id}/publicar`)}
          />
        </>
      )}

      {cual === "publicar" && <VistaPublicar importacion={importacion} />}

      {/* El estilo se abre SOLO desde las fotos, asi que su estado vive aqui y
          no en el marco: el marco no tiene por que saber que existe. */}
      {estilando !== null && (
        <SheetEstilo
          abierto
          categoria={estilando}
          idImportacion={id}
          onAbierto={(v) => !v && setEstilando(null)}
        />
      )}
    </>
  );
}

/**
 * Lo marcado: SOLO los platos que pidieron una mirada.
 *
 * Ensenar los 74 platos aqui obliga a buscar 3 entre 74. El trabajo de este
 * sub-paso es cerrar lo que no cuadra, y lo que ya cuadra no ayuda a hacerlo.
 */
function Revisar({
  idImportacion,
  marcados,
  onSeguir,
}: {
  idImportacion: string;
  marcados: Plato[];
  onSeguir: () => void;
}) {
  if (marcados.length === 0) {
    return (
      <div className="mx-auto max-w-lg py-14 text-center">
        <h2 className="font-display text-lg font-semibold">Todo cuadra</h2>
        <p className="mt-1 mb-5 text-[13.5px] text-tenue">
          Cada precio coincide con lo que dice tu carta.
        </p>
        <Boton variante="tinta" onClick={onSeguir}>
          Seguir a las fotos
          <ArrowRight className="size-4" />
        </Boton>
      </div>
    );
  }

  const bloquean = marcados.filter((p) => p.revisar?.bloquea).length;

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
          Revisa contra tu carta
        </h2>
        <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
          {bloquean > 0
            ? `${bloquean} ${bloquean === 1 ? "plato no cuadra" : "platos no cuadran"} y no se publican así. El resto solo hay que mirarlo.`
            : "Nada está roto. Solo confirma que estos están bien."}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-2.5 lg:grid-cols-[repeat(auto-fill,minmax(196px,1fr))] lg:gap-3">
        {marcados.map((plato) => (
          <TarjetaPlato
            key={plato.id}
            idImportacion={idImportacion}
            modo="marcado"
            plato={plato}
          />
        ))}
      </div>
    </div>
  );
}

function Catalogo({
  idImportacion,
  importacion,
  onEstiloDe,
}: {
  idImportacion: string;
  importacion: { categorias: { nombre: string; platos: Plato[] }[] };
  onEstiloDe: (categoria: string) => void;
}) {
  // Los ausentes salen ARRIBA y fuera de su seccion: son una decision
  // pendiente, no un plato mas del catalogo. Enterrados entre los 74 no los
  // vería nadie, y se publicarian sin que el dueno supiera que existian.
  const ausentes = importacion.categorias
    .flatMap((c) => c.platos)
    .filter((p) => p.ausente);

  return (
    <div>
      <div className="flex items-end justify-between gap-3.5">
        <div>
          <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
            Tus platos
          </h2>
          <p className="max-w-[58ch] text-[13.5px] leading-[1.5] text-[#8A867D]">
            Genera la que falte, sube la tuya o corrige la que hay.
          </p>
        </div>
        <button
          className="hidden shrink-0 items-center gap-2 rounded-full border border-borde-suave bg-surface px-[13px] py-[9px] text-[12.5px] font-medium text-tinta transition-colors hover:border-tinta lg:flex"
          type="button"
          onClick={() => onEstiloDe("")}
        >
          <span className="size-3 rounded-full border-[1.5px] border-tinta" />
          Estilo
        </button>
      </div>

      <AccionesDeFotos
        idImportacion={idImportacion}
        onEstilo={() => onEstiloDe("")}
      />

      <div className="mt-6">
        <PlatosAusentes idImportacion={idImportacion} platos={ausentes} />
      </div>

      {importacion.categorias.map((categoria) => (
        <section key={categoria.nombre} className="mb-8">
          <div className="sticky top-0 z-2 -mt-2 mb-2.5 flex items-center justify-between gap-2.5 bg-crema py-2 lg:static lg:mt-0 lg:mb-3 lg:py-0">
            <h2 className="font-display text-sm font-semibold leading-[1.2] tracking-[-0.01em]">
              {categoria.nombre}
            </h2>
            <span className="flex-1 text-[11.5px] text-tenue">
              {categoria.platos.filter((p) => !p.ausente).length}
            </span>
            <button
              className="shrink-0 rounded-full border border-borde-suave bg-surface px-[11px] py-[7px] text-[11.5px] text-muted transition-colors hover:border-tinta hover:text-tinta"
              type="button"
              onClick={() => onEstiloDe(categoria.nombre)}
            >
              Estilo
            </button>
          </div>
          <div className="grid grid-cols-1 gap-2.5 lg:grid-cols-[repeat(auto-fill,minmax(196px,1fr))] lg:gap-3">
            {categoria.platos
              .filter((plato) => !plato.ausente)
              .map((plato) => (
                <TarjetaPlato
                  key={plato.id}
                  idImportacion={idImportacion}
                  modo="fotos"
                  plato={plato}
                />
              ))}
          </div>
        </section>
      ))}
    </div>
  );
}
