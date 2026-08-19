"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
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
    <div className="flex min-h-svh flex-col md:flex-row">
      <Lateral
        idImportacion={importacion.id}
        nombre={importacion.restaurante.nombre || "Tu restaurante"}
        onEstilo={() => setEstilando("")}
        onIr={ir}
        seccion={secActiva}
        secciones={secs}
        sub={subActivo}
      />

      <main className="min-w-0 flex-1 px-4 py-6 md:px-10 md:py-10">
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
      <div className="mx-auto max-w-lg py-10 text-center">
        <h2 className="text-lg font-semibold">Todo cuadra</h2>
        <p className="mt-1 mb-5 text-sm text-muted">
          Cada precio coincide con lo que dice tu carta.
        </p>
        <Button onPress={onSeguir}>Seguir a las fotos</Button>
      </div>
    );
  }

  const bloquean = marcados.filter((p) => p.revisar?.bloquea).length;

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="text-xl font-semibold">Revisa contra tu papel</h2>
        <p className="text-sm text-muted">
          {bloquean > 0
            ? `${bloquean} ${bloquean === 1 ? "plato no cuadra" : "platos no cuadran"} y no se publican así. El resto solo hay que mirarlo.`
            : "Nada está roto. Solo confirma que estos están bien."}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
      <PlatosAusentes idImportacion={idImportacion} platos={ausentes} />

      {importacion.categorias.map((categoria) => (
        <section key={categoria.nombre} className="mb-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="text-lg font-semibold">{categoria.nombre}</h2>
            <Button
              size="sm"
              variant="tertiary"
              onPress={() => onEstiloDe(categoria.nombre)}
            >
              Estilo de esta sección
            </Button>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
    <Card variant="secondary">
      <Card.Header>
        <Card.Title>{titulo}</Card.Title>
        <Card.Description>{detalle}</Card.Description>
      </Card.Header>
      <Card.Footer>
        <Link href="/">
          <Button>Volver a intentar</Button>
        </Link>
      </Card.Footer>
    </Card>
  );
}
