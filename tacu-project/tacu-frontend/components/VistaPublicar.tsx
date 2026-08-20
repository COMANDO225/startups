"use client";

import { useMutation } from "@tanstack/react-query";
import { Check, ExternalLink } from "lucide-react";
import { API, publicarCarta, urlMedia } from "@/lib/api";
import type { Importacion } from "@/lib/tipos";
import { Aviso } from "./ui/Aviso";
import { Boton } from "./ui/Boton";
import { Ficha } from "./ui/Ficha";
import { BarraAccion } from "./BarraAccion";

/**
 * El ultimo paso: como lo vera el cliente, y el boton que da la URL.
 *
 * La vista previa es un telefono a proposito. La carta se reparte por WhatsApp y
 * se abre en el movil; ensenarla en un marco de escritorio daria una idea
 * equivocada de lo que el cliente va a ver.
 */
export function VistaPublicar({ importacion }: { importacion: Importacion }) {
  const publicar = useMutation({
    mutationFn: () => publicarCarta(importacion.id),
  });

  const slug = publicar.data?.slug ?? importacion.restaurante.slug;
  const ruta = slug ? `${API}/v1/r/${slug}` : null;
  const publicada = !!publicar.data || importacion.estado === "publicada";
  const platos = importacion.categorias.flatMap((c) => c.platos);

  return (
    <div className="mx-auto flex max-w-3xl flex-col items-center gap-6 py-4">
      <div className="text-center">
        <h2 className="font-display text-lg font-semibold leading-[1.2] tracking-[-0.02em]">
          Asi lo vera tu cliente
        </h2>
        <p className="text-sm text-muted">
          La carta se abre en el telefono, sin instalar nada.
        </p>
      </div>

      {/* El marco del telefono. Solo decoracion: dentro va el mismo catalogo. */}
      <div className="w-full max-w-[320px] overflow-hidden rounded-[2rem] border-8 border-foreground/90 bg-background shadow-xl">
        <div className="max-h-[420px] overflow-y-auto">
          <div className="px-4 py-4">
            <h3 className="text-lg font-semibold">
              {importacion.restaurante.nombre}
            </h3>
          </div>
          {importacion.categorias.map((cat) => (
            <section key={cat.nombre} className="px-4 pb-4">
              <h4 className="mb-2 text-sm font-semibold">{cat.nombre}</h4>
              <ul className="flex flex-col gap-2">
                {cat.platos.slice(0, 4).map((p) => (
                  <li key={p.id} className="flex items-center gap-2">
                    {p.foto?.url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        alt={p.nombre}
                        className="size-12 shrink-0 rounded-lg object-cover"
                        src={urlMedia(p.foto.url)}
                      />
                    ) : (
                      <span className="size-12 shrink-0 rounded-lg bg-surface-secondary" />
                    )}
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-xs">{p.nombre}</span>
                      <span className="text-xs font-semibold">{p.desde}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>

      <Ficha className="w-full p-4">
        <div>
          <p className="text-[13.5px] text-tenue">
            {platos.length} platos ·{" "}
            {platos.filter((p) => p.foto?.estado === "lista").length} con foto
          </p>

          {publicada && ruta ? (
            <div className="flex flex-col gap-2">
              <Aviso tono="neutro" titulo={<>Tu carta esta publicada</>}>
                Comparte este enlace por WhatsApp o ponlo en un QR.
              </Aviso>
              <a
                className="flex items-center gap-2 rounded-lg border border-default px-3 py-2 font-mono text-sm break-all"
                href={ruta}
                rel="noreferrer"
                target="_blank"
              >
                <ExternalLink className="size-4 shrink-0" />
                {ruta}
              </a>
            </div>
          ) : (
            <>
              {!importacion.puede_publicarse && (
                <Aviso tono="confirmar">
                  Quedan {importacion.marcas.revisar} platos por corregir en el
                  paso Tu catálogo.
                </Aviso>
              )}
              {publicar.error && (
                <p className="text-sm text-bloquea">{publicar.error.message}</p>
              )}
              <Boton
                ancho
                className="hidden lg:flex"
                disabled={!importacion.puede_publicarse || publicar.isPending}
                tamano="lg"
                variante="amarillo"
                onClick={() => publicar.mutate()}
              >
                <Check className="size-4" />
                {publicar.isPending ? "Publicando..." : "Publicar mi carta"}
              </Boton>
            </>
          )}
        </div>
      </Ficha>

      {!publicada && (
        <BarraAccion
          disabled={!importacion.puede_publicarse || publicar.isPending}
          etiqueta={publicar.isPending ? "Publicando…" : "Publicar mi carta"}
          nota={
            importacion.puede_publicarse
              ? undefined
              : `Te faltan ${importacion.marcas.revisar} platos por corregir.`
          }
          variante={importacion.puede_publicarse ? "amarillo" : "peligro"}
          onClick={() => publicar.mutate()}
        />
      )}
    </div>
  );
}
