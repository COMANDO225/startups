import type {
  ErrorApi,
  EstadoFotos,
  Estilo,
  Importacion,
  ImportacionCreada,
  Pagina,
  PlatoEditado,
  Publicada,
  Referencias,
  TipoRestaurante,
  TiposDeNegocio,
} from "./tipos";

export const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** Las urls de /media vienen relativas al backend, que no es el mismo origen que el front. */
export function urlMedia(url: string | undefined): string | undefined {
  return url ? API + url : undefined;
}

export class FalloApi extends Error {
  constructor(
    readonly codigo: number,
    message: string,
    readonly detalle?: string,
  ) {
    super(message);
  }
  get sinToken() {
    return this.codigo === 401;
  }
  get noEncontrada() {
    return this.codigo === 404;
  }
  get peticionMala() {
    return this.codigo === 400;
  }
}

// --- token ---
// Viaja UNA sola vez, en la respuesta de crear. Si se pierde, el borrador es
// inaccesible para siempre: se guarda antes de navegar a ningun sitio.

const clave = (id: string) => `tacu.token.${id}`;

export function guardarToken(id: string, token: string) {
  localStorage.setItem(clave(id), token);
}

export function obtenerToken(id: string): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(clave(id));
}

// --- llamadas ---

async function pedir<T>(
  ruta: string,
  opciones: RequestInit & { token?: string | null } = {},
): Promise<T> {
  const { token, headers, ...resto } = opciones;
  const r = await fetch(API + ruta, {
    ...resto,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...headers,
    },
  });

  if (!r.ok) {
    // El cuerpo de error puede no ser JSON (un 502 del proxy, por ejemplo).
    const cuerpo = (await r.json().catch(() => null)) as ErrorApi | null;
    throw new FalloApi(
      r.status,
      cuerpo?.error ?? `error ${r.status}`,
      cuerpo?.detalle,
    );
  }

  if (r.status === 204) return undefined as T;
  return (await r.json()) as T;
}

function conToken(id: string): string {
  const token = obtenerToken(id);
  if (!token) {
    throw new FalloApi(
      401,
      "no tenemos el acceso a este borrador en este dispositivo",
    );
  }
  return token;
}

/**
 * Crea el restaurante: nombre y tipo de negocio, todavia sin carta.
 *
 * El token que devuelve ES la sesion —no hay contrasena— y se guarda antes de
 * volver, porque viaja UNA sola vez: si se navega antes de guardarlo, el dueno
 * pierde el restaurante que acaba de crear.
 */
export async function crearRestaurante(
  nombre: string,
  tipos: string[],
): Promise<ImportacionCreada> {
  const cuerpo = new FormData();
  cuerpo.set("nombre", nombre);
  for (const tipo of tipos) cuerpo.append("tipos", tipo);

  const creada = await pedir<ImportacionCreada>("/v1/importaciones", {
    method: "POST",
    body: cuerpo,
  });
  guardarToken(creada.id, creada.token);
  return creada;
}

/** Las hojas de la carta, ya con el restaurante creado. Arranca la lectura. */
export function subirCarta(
  id: string,
  fotos: File[],
): Promise<{ estado: string }> {
  const cuerpo = new FormData();
  for (const foto of fotos) cuerpo.append("fotos", foto);

  return pedir(`/v1/importaciones/${id}/carta`, {
    method: "POST",
    token: conToken(id),
    body: cuerpo,
  });
}

/** Borra un plato de verdad. Lo decide el dueno; ninguna lectura borra nada. */
export function quitarPlato(
  idImportacion: string,
  idPlato: string,
): Promise<void> {
  return pedir(`/v1/platos/${idPlato}`, {
    method: "DELETE",
    token: conToken(idImportacion),
  });
}

/** Deshace el "ausente": sigue en la carta aunque la lectura no lo trajera. */
export function recuperarPlato(
  idImportacion: string,
  idPlato: string,
): Promise<void> {
  return pedir(`/v1/platos/${idPlato}/recuperar`, {
    method: "POST",
    token: conToken(idImportacion),
  });
}

/**
 * Vuelve a leer la carta ENTERA desde las hojas guardadas.
 *
 * Ya NO destruye: la lectura cruza por nombre y el plato que sigue en la carta
 * conserva su id, o sea su foto y las correcciones del dueno.
 */
export function releerCarta(id: string): Promise<{ estado: string }> {
  return pedir(`/v1/importaciones/${id}/carta/releer`, {
    method: "POST",
    token: conToken(id),
  });
}

/** El catalogo de tipos sin restaurante detras: lo pide la primera pantalla. */
export function catalogoDeTipos(): Promise<{ catalogo: TipoRestaurante[] }> {
  return pedir("/v1/tipos");
}

export function obtenerImportacion(id: string): Promise<Importacion> {
  return pedir<Importacion>(`/v1/importaciones/${id}`, { token: conToken(id) });
}

export function obtenerFotos(id: string): Promise<EstadoFotos> {
  return pedir<EstadoFotos>(`/v1/importaciones/${id}/fotos`, {
    token: conToken(id),
  });
}

/**
 * corregir edita la foto actual con el ajuste guardado en vez de hacer una
 * nueva. Solo con platos concretos.
 */
export function generarFotos(
  id: string,
  que: ({ todos: true } | { platos: string[] }) & { corregir?: boolean },
): Promise<{ encoladas: number }> {
  return pedir(`/v1/importaciones/${id}/fotos`, {
    method: "POST",
    token: conToken(id),
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(que),
  });
}

// El token es de la importacion aunque la ruta sea de un plato.
export function subirFotoDePlato(
  idImportacion: string,
  idPlato: string,
  foto: File,
): Promise<void> {
  const cuerpo = new FormData();
  cuerpo.set("foto", foto);
  return pedir(`/v1/platos/${idPlato}/foto`, {
    method: "POST",
    token: conToken(idImportacion),
    body: cuerpo,
  });
}

export function quitarFotoDePlato(
  idImportacion: string,
  idPlato: string,
): Promise<void> {
  return pedir(`/v1/platos/${idPlato}/foto`, {
    method: "DELETE",
    token: conToken(idImportacion),
  });
}

/**
 * Guarda la correccion del dueno para la foto de un plato. NO regenera.
 *
 * Son dos llamadas a proposito: reescribir el texto es gratis y regenerar
 * cuesta $0.0336, asi que el dueno afina primero y paga cuando le convence.
 */
export function ajustarFotoDePlato(
  idImportacion: string,
  idPlato: string,
  ajuste: string,
): Promise<void> {
  return pedir(`/v1/platos/${idPlato}/foto-ajuste`, {
    method: "PATCH",
    token: conToken(idImportacion),
    body: JSON.stringify({ ajuste }),
    headers: { "Content-Type": "application/json" },
  });
}

// --- edicion de platos ---

/** Le pone nombre a los precios de un plato. Es lo que desbloquea publicar. */
export function editarEtiquetas(
  idImportacion: string,
  idPlato: string,
  etiquetas: string[],
): Promise<PlatoEditado> {
  return pedir(`/v1/platos/${idPlato}`, {
    method: "PATCH",
    token: conToken(idImportacion),
    body: JSON.stringify({ etiquetas }),
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Renombra el restaurante. NO mueve la direccion publicada: el slug se recalcula
 * al publicar, asi que la URL que el dueno ya repartio sigue viva hasta que el
 * decida republicar.
 */
export function guardarNombre(id: string, nombre: string): Promise<void> {
  return pedir(`/v1/importaciones/${id}/nombre`, {
    method: "PUT",
    token: conToken(id),
    body: JSON.stringify({ nombre }),
    headers: { "Content-Type": "application/json" },
  });
}

// --- paginas de la carta ---

/**
 * Anade una hoja y la manda a leer. Responde 202: cuando vuelve, la pagina esta
 * guardada pero sus platos todavia no. paginas_leyendo dice cuando llegaron.
 *
 * Lee SOLO esa hoja y suma lo que no estaba. Releer la carta entera borraria los
 * platos y con ellos las fotos ya generadas.
 */
export function agregarPagina(
  id: string,
  archivo: File,
): Promise<{ paginas: Pagina[] }> {
  const cuerpo = new FormData();
  cuerpo.set("pagina", archivo);
  return pedir(`/v1/importaciones/${id}/paginas`, {
    method: "POST",
    token: conToken(id),
    body: cuerpo,
  });
}

/** Saca la hoja de la lista. Los platos que ya se leyeron de ella se quedan. */
export function quitarPaginaConPlatos(
  id: string,
  clave: string,
): Promise<{ platos_borrados: number }> {
  return pedir(
    `/v1/importaciones/${id}/paginas?clave=${encodeURIComponent(clave)}&platos=borrar`,
    { method: "DELETE", token: conToken(id) },
  );
}

export function quitarPagina(
  id: string,
  clave: string,
): Promise<{ paginas: Pagina[] }> {
  return pedir(
    `/v1/importaciones/${id}/paginas?clave=${encodeURIComponent(clave)}`,
    {
      method: "DELETE",
      token: conToken(id),
    },
  );
}

export function reordenarPaginas(
  id: string,
  claves: string[],
): Promise<{ paginas: Pagina[] }> {
  return pedir(`/v1/importaciones/${id}/paginas`, {
    method: "PUT",
    token: conToken(id),
    body: JSON.stringify({ claves }),
    headers: { "Content-Type": "application/json" },
  });
}

// --- estilo ---

/** Con categoria devuelve la base de esa seccion, ya plegada sobre la general. */
export function obtenerEstilo(id: string, categoria = ""): Promise<Estilo> {
  const q = categoria ? `?categoria=${encodeURIComponent(categoria)}` : "";
  return pedir(`/v1/importaciones/${id}/estilo${q}`, { token: conToken(id) });
}

export function guardarEstilo(
  id: string,
  categoria: string,
  base: { recipiente: string; fondo: string },
): Promise<void> {
  const q = categoria ? `?categoria=${encodeURIComponent(categoria)}` : "";
  return pedir(`/v1/importaciones/${id}/estilo${q}`, {
    method: "PUT",
    token: conToken(id),
    body: JSON.stringify(base),
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Los tipos de negocio con el reparto de la carta: cuantos platos toma cada uno,
 * cuantos no se parecen a ninguno y que tipo se esta quedando fuera.
 */
export function obtenerTipos(id: string): Promise<TiposDeNegocio> {
  return pedir(`/v1/importaciones/${id}/tipos`, { token: conToken(id) });
}

/**
 * Los tipos de negocio, en orden: el primero es el principal y es el que manda
 * en un plato que no se parece a ninguno. Son varios porque un negocio peruano
 * suele ser varios: la cevicheria que tambien vende pollo a la brasa.
 */
export function guardarTipos(id: string, tipos: string[]): Promise<void> {
  return pedir(`/v1/importaciones/${id}/tipos`, {
    method: "PUT",
    token: conToken(id),
    body: JSON.stringify({ tipos }),
    headers: { "Content-Type": "application/json" },
  });
}

// --- fotos de ejemplo ---

function conArchivo(foto: File) {
  const cuerpo = new FormData();
  cuerpo.set("foto", foto);
  return cuerpo;
}

export function subirReferenciaDePlato(
  idImportacion: string,
  idPlato: string,
  foto: File,
): Promise<Referencias> {
  return pedir(`/v1/platos/${idPlato}/referencias`, {
    method: "POST",
    token: conToken(idImportacion),
    body: conArchivo(foto),
  });
}

export function quitarReferenciaDePlato(
  idImportacion: string,
  idPlato: string,
  clave: string,
): Promise<Referencias> {
  return pedir(
    `/v1/platos/${idPlato}/referencias?clave=${encodeURIComponent(clave)}`,
    { method: "DELETE", token: conToken(idImportacion) },
  );
}

/** Dibuja el recipiente vacio del estilo. CUESTA una foto del presupuesto. */
export function generarVistaDeEstilo(
  id: string,
  categoria: string,
): Promise<{ vista: string }> {
  const q = categoria ? `?categoria=${encodeURIComponent(categoria)}` : "";
  return pedir(`/v1/importaciones/${id}/estilo/vista${q}`, {
    method: "POST",
    token: conToken(id),
  });
}

export function subirReferenciaDeBase(
  id: string,
  categoria: string,
  foto: File,
): Promise<Referencias> {
  const q = categoria ? `?categoria=${encodeURIComponent(categoria)}` : "";
  return pedir(`/v1/importaciones/${id}/estilo/referencias${q}`, {
    method: "POST",
    token: conToken(id),
    body: conArchivo(foto),
  });
}

export function quitarReferenciaDeBase(
  id: string,
  categoria: string,
  clave: string,
): Promise<Referencias> {
  const p = new URLSearchParams({ clave });
  if (categoria) p.set("categoria", categoria);
  return pedir(`/v1/importaciones/${id}/estilo/referencias?${p}`, {
    method: "DELETE",
    token: conToken(id),
  });
}

// --- publicar ---

/** Deja la carta visible en /r/{slug}. 409 si quedan platos en rojo. */
export function publicarCarta(id: string): Promise<Publicada> {
  return pedir(`/v1/importaciones/${id}/publicar`, {
    method: "POST",
    token: conToken(id),
  });
}

/** La carta publica NO lleva token: es la que ve el cliente del restaurante. */
export function obtenerCartaPublica(slug: string): Promise<Importacion> {
  return pedir(`/v1/r/${encodeURIComponent(slug)}`);
}
