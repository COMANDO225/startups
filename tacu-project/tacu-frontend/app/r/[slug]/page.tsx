import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { API } from "@/lib/api";
import type { CartaPublica } from "@/lib/tipos";

/**
 * La carta publica. Server Component: CERO JavaScript de cliente.
 *
 * Es la unica pagina que ve el comensal, se abre desde un enlace de WhatsApp en
 * un telefono de gama baja con datos moviles, y no tiene ni un boton. Mandarle
 * React para pintar una lista seria cobrarle megas por nada.
 */
async function cartaDe(slug: string): Promise<CartaPublica | null> {
  const r = await fetch(`${API}/v1/r/${encodeURIComponent(slug)}`, {
    // La carta cambia cuando el dueno republica, no en cada visita.
    next: { revalidate: 60 },
  });
  if (!r.ok) return null;
  return r.json();
}

export async function generateMetadata({
  params,
}: PageProps<"/r/[slug]">): Promise<Metadata> {
  const { slug } = await params;
  const carta = await cartaDe(slug);
  if (!carta) return { title: "Carta no encontrada" };

  const titulo = `${carta.restaurante.nombre} · Carta`;
  const primera = carta.categorias
    .flatMap((c) => c.platos)
    .find((p) => p.foto?.url);

  // Open Graph: la carta se reparte por WhatsApp, y ahi la miniatura y el
  // titulo son lo unico que se ve antes de abrirla.
  return {
    title: titulo,
    description: `Mira la carta de ${carta.restaurante.nombre}.`,
    openGraph: {
      title: titulo,
      description: `Mira la carta de ${carta.restaurante.nombre}.`,
      images: primera?.foto?.url ? [`${API}${primera.foto.url}`] : undefined,
    },
  };
}

export default async function CartaPublica({ params }: PageProps<"/r/[slug]">) {
  const { slug } = await params;
  const carta = await cartaDe(slug);
  if (!carta) notFound();

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-6">
      <h1 className="text-2xl font-semibold">{carta.restaurante.nombre}</h1>

      {carta.categorias.map((categoria) => (
        <section key={categoria.nombre} className="mt-6">
          <h2 className="mb-3 text-lg font-semibold">{categoria.nombre}</h2>
          <ul className="flex flex-col gap-3">
            {categoria.platos.map((plato) => (
              <li key={plato.id} className="flex items-start gap-3">
                {plato.foto?.url_pequena ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    alt={plato.nombre}
                    className="size-20 shrink-0 rounded-xl object-cover"
                    // loading lazy: 74 fotos de golpe en datos moviles no se
                    // descargan, se abandonan.
                    loading="lazy"
                    // La de 320 y no la grande: se pinta a 80 px, y esta pagina
                    // se abre desde WhatsApp con datos moviles. Medido: 8.9 KB
                    // contra 46.
                    src={`${API}${plato.foto.url_pequena}`}
                  />
                ) : (
                  <span className="size-20 shrink-0 rounded-xl bg-surface-secondary" />
                )}

                <span className="min-w-0 flex-1">
                  <span className="block font-medium">{plato.nombre}</span>
                  {plato.descripcion && (
                    <span className="block text-sm text-muted">
                      {plato.descripcion}
                    </span>
                  )}
                  <span className="mt-1 flex flex-wrap gap-x-3 gap-y-1">
                    {plato.precios.map((precio, i) => (
                      <span key={i} className="text-sm">
                        {precio.etiqueta && (
                          <span className="text-muted">{precio.etiqueta} </span>
                        )}
                        <span className="font-semibold">{precio.soles}</span>
                      </span>
                    ))}
                  </span>
                </span>
              </li>
            ))}
          </ul>
        </section>
      ))}

      <p className="mt-10 text-center text-xs text-muted">
        Carta hecha con Tacu
      </p>
    </main>
  );
}
