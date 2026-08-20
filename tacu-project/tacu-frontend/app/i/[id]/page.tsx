"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Boton } from "@/components/ui/Boton";
import { Ficha } from "@/components/ui/Ficha";
import { AccionesDeFotos } from "@/components/AccionesDeFotos";
import { CatalogoEsqueleto } from "@/components/CatalogoEsqueleto";
import { PlatosAusentes } from "@/components/PlatosAusentes";
import { Lateral } from "@/components/Lateral";
import { SheetEstilo } from "@/components/SheetEstilo";
import { TarjetaPlato } from "@/components/TarjetaPlato";
import { VistaCarta } from "@/components/VistaCarta";
import { VistaDatos } from "@/components/VistaDatos";
import { VistaPublicar } from "@/components/VistaPublicar";
import { FalloApi } from "@/lib/api";
import { secciones as armarSecciones } from "@/lib/flujo";
import { useFotos, useImportacion } from "@/lib/hooks";
import type { Plato } from "@/lib/tipos";

export default function Trabajo() {
  const { id } = useParams<{ id: string }>();
  const { data: importacion, error, isPending } = useImportacion(id);
  const { data: fotos } = useFotos(id);
  const router = useRouter();

  // Donde esta el dueno: seccion y sub-paso.
  const [seccion, setSeccion] = useState("carta");
  const [sub, setSub] = useState("revisar");
  const [estilando, setEstilando] = useState<string | null>(null);

  if (isPending)
    return (
      <Suelto>
        <CatalogoEsqueleto />
      </Suelto>
    );

  if (error) {
    const fallo = error instanceof FalloApi ? error : null;
    const sinAcceso = fallo?.sinToken || fallo?.noEncontrada;
    return (
      <Suelto>
        <Aviso
          detalle={
            sinAcceso
              ? "El acceso se guarda en el navegador donde la subiste. Desde otro equipo hay que subirla de nuevo."
              : error.message
          }
          titulo={
            sinAcceso
              ? "Este borrador no se abre en este teléfono"
              : "No pudimos abrir tu carta"
          }
        />
      </Suelto>
    );
  }

  const lista = Object.values(fotos?.fotos ?? {});
  const secs = armarSecciones({
    importacion,
    conFoto: lista.filter((f) => f.estado === "lista").length,
    platos: lista.length,
  });

  // Mientras no haya carta leida, lo unico que hay que mirar es la carta:
  // 'nueva' es el restaurante recien creado y le toca subir sus hojas,
  // 'leyendo' es la lectura en vuelo, y 'fallida' es la que hay que repetir.
  //
  // 'fallida' hacia falta: sin ella el dueno caia en "Revisar precios" —una
  // pantalla sin un solo plato— en vez de en el aviso que le dice que su foto
  // no se pudo leer y como repetirla.
  const leyendo = importacion.estado === "leyendo";
  const sinCarta =
    leyendo ||
    importacion.estado === "nueva" ||
    importacion.estado === "fallida";
  const secActiva = sinCarta ? "restaurante" : seccion;
  const subActivo = sinCarta ? "carta" : sub;

  const ir = (s: string, sb?: string) => {
    const destino = secs.find((x) => x.id === s);
    if (!destino || destino.bloqueada) return;
    setSeccion(s);
    setSub(sb ?? destino.subs[0]?.id ?? "");
  };

  const platos = importacion.categorias.flatMap((c) => c.platos);
  const marcados = platos.filter((p) => p.revisar);

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      <Lateral
        estado={importacion.estado}
        // Lo que va a la derecha del titulo en el telefono: el dato de la
        // seccion en la que estas, no un recuento global.
        meta={
          secActiva === "restaurante"
            ? `${importacion.paginas.length} ${importacion.paginas.length === 1 ? "hoja" : "hojas"}`
            : secActiva === "carta"
              ? `${platos.length} platos`
              : importacion.estado === "publicada"
                ? "en línea"
                : undefined
        }
        nombre={importacion.restaurante.nombre || "Tu restaurante"}
        seccion={secActiva}
        secciones={secs}
        sub={subActivo}
        onIr={ir}
      />

      <main className="anima-panel min-w-0 flex-1 px-[14px] pt-4 pb-24 lg:max-w-[1080px] lg:px-8 lg:pt-[26px] lg:pb-10">
        {secActiva === "restaurante" && subActivo === "datos" && (
          <VistaDatos
            importacion={importacion}
            onSeguir={() => ir("restaurante", "carta")}
          />
        )}

        {secActiva === "restaurante" && subActivo === "carta" && (
          <VistaCarta
            conFoto={lista.filter((f) => f.estado === "lista").length}
            importacion={importacion}
            onReintentar={() => router.push("/")}
          />
        )}

        {secActiva === "carta" && subActivo === "revisar" && (
          <Revisar
            idImportacion={id}
            marcados={marcados}
            onSeguir={() => ir("carta", "fotos")}
          />
        )}

        {secActiva === "carta" && subActivo === "fotos" && (
          <Catalogo
            idImportacion={id}
            importacion={importacion}
            onEstiloDe={setEstilando}
          />
        )}

        {secActiva === "publicar" && (
          <VistaPublicar importacion={importacion} />
        )}
      </main>

      {estilando !== null && (
        <SheetEstilo
          abierto
          categoria={estilando}
          idImportacion={id}
          onAbierto={(v) => !v && setEstilando(null)}
        />
      )}
    </div>
  );
}

/**
 * Revisar precios: SOLO lo marcado.
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
          Seguir a las fotos →
        </Boton>
      </div>
    );
  }

  const bloquean = marcados.filter((p) => p.revisar?.bloquea).length;

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="hidden font-display text-xl font-semibold leading-[1.2] tracking-[-0.02em] lg:block">
          Revisa contra tu papel
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
            modo="datos"
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

function Suelto({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10">{children}</main>
  );
}

function Aviso({ titulo, detalle }: { titulo: string; detalle: string }) {
  return (
    <Ficha className="p-5">
      <h2 className="font-display text-lg font-semibold">{titulo}</h2>
      <p className="mt-1.5 mb-5 text-[13.5px] leading-[1.5] text-tenue">
        {detalle}
      </p>
      <div>
        <Link href="/">
          <Boton variante="tinta">Volver a intentar</Boton>
        </Link>
      </div>
    </Ficha>
  );
}
