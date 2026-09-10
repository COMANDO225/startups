"use client";

import { useEffect } from "react";
import Link from "next/link";
import {
  useParams,
  useRouter,
  useSelectedLayoutSegment,
} from "next/navigation";
import { EsqueletoDePaso } from "@/components/EsqueletoDePaso";
import { Lateral } from "@/components/Lateral";
import { Boton } from "@/components/ui/Boton";
import { Ficha } from "@/components/ui/Ficha";
import { FalloApi, recordarRestaurante } from "@/lib/api";
import { alcance, DONDE, pasoValido, puedeIr, secciones } from "@/lib/flujo";
import { useFotos, useImportacion } from "@/lib/hooks";

/**
 * El marco del editor y EL GUARD.
 *
 * Cada paso es una ruta —/i/{id}/{paso}—, asi que atras y adelante del navegador
 * mueven entre pasos en vez de sacarte de la aplicacion, y un paso se puede
 * enlazar y recargar. Antes el recorrido vivia en dos useState: el candado del
 * stepper era pintura, no regla, y solo existia mientras ese componente
 * estuviera montado.
 *
 * Quien decide si un paso se puede es flujo.puedeIr, la MISMA funcion que pinta
 * el candado. Aqui solo se aplica.
 */
export default function MarcoDelEditor({
  children,
}: {
  children: React.ReactNode;
}) {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const segmento = useSelectedLayoutSegment();
  const paso = pasoValido(segmento);

  const { data: importacion, error, isPending } = useImportacion(id);

  // El token ya lo recogio obtenerToken; aqui solo se quita de la barra, para
  // que no quede en el historial ni en lo que el dueno copie luego para ensenar
  // su carta. replaceState y no router.replace: no hace falta navegar.
  useEffect(() => {
    if (window.location.search) {
      window.history.replaceState({}, "", window.location.pathname);
    }
  }, []);

  // Si la carta se puede leer, se recuerda. Cubre a quien llega con el enlace de
  // recuperacion y a quien tiene el token pero perdio el indice: la pantalla de
  // inicio le ofrecia crear una carta nueva teniendo la suya delante.
  useEffect(() => {
    if (importacion) recordarRestaurante(id, importacion.restaurante.nombre);
  }, [id, importacion]);
  const { data: fotos } = useFotos(id);

  // El guard. En un efecto y no durante el render: redirigir mientras se pinta
  // deja a React a medias y avisa por consola.
  useEffect(() => {
    if (isPending || error || !importacion) return;
    if (paso && puedeIr(paso, importacion)) return;
    router.replace(`/i/${id}/${alcance(importacion)}`);
  }, [error, id, importacion, isPending, paso, router]);

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

  // Mientras carga o mientras el guard redirige, el marco se queda y solo el
  // contenido es un hueco. Antes desaparecia entero —rail, cabecera y todo— y
  // volvia medio segundo despues: eso era el parpadeo.
  const esperando = isPending || !paso || !puedeIr(paso, importacion);

  const lista = Object.values(fotos?.fotos ?? {});
  const secs = secciones({
    importacion: importacion ?? undefined,
    conFoto: lista.filter((f) => f.estado === "lista").length,
    platos: lista.length,
  });
  const platos = importacion?.categorias.flatMap((c) => c.platos) ?? [];
  const donde = DONDE[paso ?? "datos"];

  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      <Lateral
        cargando={isPending}
        estado={importacion?.estado ?? "nueva"}
        // Lo que va a la derecha del titulo en el telefono: el dato de la
        // seccion en la que estas, no un recuento global.
        meta={
          !importacion
            ? undefined
            : donde.seccion === "restaurante"
              ? `${importacion.paginas.length} ${importacion.paginas.length === 1 ? "hoja" : "hojas"}`
              : donde.seccion === "carta"
                ? `${platos.length} platos`
                : importacion.estado === "publicada"
                  ? "en línea"
                  : undefined
        }
        nombre={importacion?.restaurante.nombre || "Tu restaurante"}
        seccion={donde.seccion}
        secciones={secs}
        sub={donde.sub}
        onIr={(s, sb) => {
          // La columna habla de secciones; las rutas, de pasos.
          const destino = secs.find((x) => x.id === s);
          if (!destino || destino.bloqueada) return;
          const elegido = sb ?? destino.subs[0]?.id ?? "";
          const ruta = (Object.keys(DONDE) as (keyof typeof DONDE)[]).find(
            (p) => DONDE[p].seccion === s && DONDE[p].sub === elegido,
          );
          if (ruta) router.push(`/i/${id}/${ruta}`);
        }}
      />

      <main className="anima-panel min-w-0 flex-1 px-[14px] pt-4 pb-[calc(7.5rem+env(safe-area-inset-bottom))] lg:max-w-[1080px] lg:px-8 lg:pt-[26px] lg:pb-10">
        {esperando ? <EsqueletoDePaso paso={paso} /> : children}
      </main>
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
