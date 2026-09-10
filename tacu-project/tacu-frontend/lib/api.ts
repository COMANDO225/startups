import type {
  ErrorApi,
  EstadoFotos,
  CualRanura,
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

/**
 * LA carta de este navegador. Una, no una lista.
 *
 * Un "crear" hace restaurante E importacion nuevos, y actualizar el menu es
 * editar LA MISMA importacion —anadir hojas, releer—, nunca crear otra. Asi que
 * varias entradas aqui nunca fueron varios restaurantes: eran los intentos que
 * el dueno empezo y abandono, acumulandose sin forma de quitarlos. Tener varios
 * locales de verdad son SEDES, y eso es otra cosa que no es este MVP.
 *
 * Sin esto, volver a una carta dependia del historial del navegador: el token
 * estaba guardado por id, pero nada lo enumeraba, y perder la URL era perder la
 * carta aunque el acceso siguiera ahi.
 *
 * Guarda el nombre para no llamar al backend solo para pintar un enlace.
 */
const CLAVE_CARTA = "tacu.restaurantes";

export type RestauranteRecordado = { id: string; nombre: string };

export function recordarRestaurante(id: string, nombre: string) {
  localStorage.setItem(CLAVE_CARTA, JSON.stringify([{ id, nombre }]));
  for (const avisar of oyentes) avisar();
}

/**
 * La instantanea del SERVIDOR, que React usa TAMBIEN en el render de
 * hidratacion —y ese ocurre en el navegador, donde window ya existe—. Por eso
 * no vale miRestaurante aunque mire `typeof window`: devolvia lo guardado
 * contra un HTML generado sin nada, y React tiraba la rama entera.
 */
export function sinRestaurante(): RestauranteRecordado | null {
  return null;
}

/**
 * Se cachea contra su texto crudo, y NO es optimizacion: quien lo lee es
 * useSyncExternalStore, que compara la instantanea por identidad. Un objeto
 * nuevo en cada llamada lo hace re-renderizar sin parar.
 */
let crudoCache = "";
let valorCache: RestauranteRecordado | null = null;

export function miRestaurante(): RestauranteRecordado | null {
  if (typeof window === "undefined") return null;
  const crudo = localStorage.getItem(CLAVE_CARTA) ?? "";
  if (crudo === crudoCache) return valorCache;
  crudoCache = crudo;
  valorCache = interpretar(crudo);
  return valorCache;
}

function interpretar(crudo: string): RestauranteRecordado | null {
  try {
    const leido: unknown = JSON.parse(crudo);
    // Se guardaba como lista y hay navegadores con varias dentro: se queda la
    // primera, que es la ultima tocada. Las demas se caen solas, sin migracion
    // ni cambiar la clave. Una entrada rota se trata como que no hay.
    const uno = Array.isArray(leido) ? leido[0] : leido;
    return uno && typeof uno.id === "string" && typeof uno.nombre === "string"
      ? { id: uno.id, nombre: uno.nombre }
      : null;
  } catch {
    return null;
  }
}

/** Quien esta pintando el enlace. Se avisa al escribir en ESTA pestana; las
 *  otras se enteran por el evento `storage` del navegador. */
const oyentes = new Set<() => void>();

export function escucharCarta(avisar: () => void) {
  oyentes.add(avisar);
  window.addEventListener("storage", avisar);
  return () => {
    oyentes.delete(avisar);
    window.removeEventListener("storage", avisar);
  };
}

export function obtenerToken(id: string): string | null {
  if (typeof window === "undefined") return null;
  // El de la URL PRIMERO, y no como respaldo del guardado: quien llega con un
  // enlace de recuperacion trae la llave en la mano, y el enlace existe
  // exactamente para cuando la guardada ya no vale. Al reves —guardado primero—
  // un token viejo se quedaba delante del bueno para siempre y el enlace no
  // servia de nada. Paso de verdad.
  return deLaURL(id) ?? localStorage.getItem(clave(id));
}

/**
 * EL ENLACE DE RECUPERACION: /i/{id}?t={token}.
 *
 * Sin registro y sin contrasena el token es lo unico que abre una carta, vive en
 * UN navegador y viaja una sola vez. Perderlo —cambiar de telefono, limpiar el
 * navegador, entrar desde otro equipo— dejaba la carta inalcanzable para
 * siempre, con sus fotos ya pagadas dentro, y la unica salida era escribir en
 * localStorage desde la consola. Un dueno de restaurante no tiene consola.
 *
 * Se resuelve AQUI y no en un efecto de la pantalla porque el token hace falta
 * ANTES de la primera peticion: el marco del editor consulta la carta en su
 * primer render, y un efecto llega tarde —salia el cartel de "este borrador no
 * se abre en este telefono" y ahi se acababa el intento—.
 *
 * Se guarda al leerlo, asi que la siguiente visita ya no depende del enlace.
 *
 * Contrapartida asumida: el token viaja en la URL y una URL se comparte sin
 * pensarlo. A cambio, hoy la alternativa es perder la carta entera. El marco
 * limpia la barra de direcciones en cuanto entra.
 */
// La query de la PRIMERA carga, copiada al importar el modulo.
//
// El marco limpia la barra en cuanto entra —el token no tiene por que quedarse
// en el historial—, y ese limpiado corria ANTES que la primera peticion: se
// llevaba el token por delante y volvia a salir el cartel de "no se abre en este
// telefono". Con la copia no hay carrera que perder.
const entrada =
  typeof window === "undefined"
    ? null
    : { ruta: window.location.pathname, busqueda: window.location.search };

function deLaURL(id: string): string | null {
  // La ruta tiene que ser la de ESTA carta: sin esto, entrar con el enlace de
  // una y navegar despues a otra le habria guardado a la segunda el token de la
  // primera, y ahi la ruptura no se ve hasta mucho despues.
  if (!entrada || !entrada.ruta.includes(id)) return null;
  const t = new URLSearchParams(entrada.busqueda).get("t");
  if (!t) return null;
  guardarToken(id, t);
  return t;
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
  recordarRestaurante(creada.id, nombre);
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

/**
 * Corrige a mano las opciones de precio de un plato: el nombre de cada una y su
 * importe.
 *
 * Las dos listas van por POSICION, como se pintan. Un importe en blanco deja el
 * que ya estaba; se manda como TEXTO porque lo parsea el backend con el mismo
 * lector que cruza los precios de la carta, asi que "45", "45.50" y "S/ 45"
 * valen igual.
 */
export function editarPrecios(
  idImportacion: string,
  idPlato: string,
  etiquetas: string[],
  importes: string[],
): Promise<PlatoEditado> {
  return pedir(`/v1/platos/${idPlato}`, {
    method: "PATCH",
    token: conToken(idImportacion),
    body: JSON.stringify({ etiquetas, importes }),
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Renombra el restaurante. NO mueve la direccion publicada: el slug se recalcula
 * al publicar, asi que la URL que el dueno ya repartio sigue viva hasta que el
 * decida republicar.
 */
export function guardarNombre(id: string, nombre: string): Promise<void> {
  recordarRestaurante(id, nombre);
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
  return pedir(`/v1/importaciones/${id}/estilo${enCategoria(categoria)}`, {
    token: conToken(id),
  });
}

/** La categoria vacia es la general, y entonces no se manda el parametro. */
function enCategoria(categoria: string): string {
  return categoria ? `?categoria=${encodeURIComponent(categoria)}` : "";
}

/**
 * Guarda los TEXTOS de las dos ranuras. Las fotos no entran por aqui: van por su
 * propia ruta, o corregir una palabra del texto borraria la foto.
 *
 * Devuelve el estilo entero ya plegado, como todo lo que escribe aqui: la
 * pantalla ensena las dos ranuras a la vez y con un 204 tendria que volver a
 * pedirlo para pintar lo que acaba de hacer.
 */
export function guardarEstilo(
  id: string,
  categoria: string,
  textos: { vajilla: string; fondo: string },
): Promise<Estilo> {
  return pedir(`/v1/importaciones/${id}/estilo${enCategoria(categoria)}`, {
    method: "PUT",
    token: conToken(id),
    body: JSON.stringify(textos),
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

/** Dibuja lo que el dueno escribio en una ranura. CUESTA una foto del presupuesto. */
export function dibujarRanura(
  id: string,
  categoria: string,
  ranura: CualRanura,
): Promise<Estilo> {
  return pedir(
    `/v1/importaciones/${id}/estilo/${ranura}/dibujo${enCategoria(categoria)}`,
    { method: "POST", token: conToken(id) },
  );
}

/** La foto que el dueno tomo de su plato o de su mesa. Gratis e instantanea. */
export function subirFotoDeRanura(
  id: string,
  categoria: string,
  ranura: CualRanura,
  foto: File,
): Promise<Estilo> {
  return pedir(
    `/v1/importaciones/${id}/estilo/${ranura}/foto${enCategoria(categoria)}`,
    { method: "POST", token: conToken(id), body: conArchivo(foto) },
  );
}

/** Devuelve la ranura al de siempre: sin texto, sin foto y sin dibujo. */
export function vaciarRanura(
  id: string,
  categoria: string,
  ranura: CualRanura,
): Promise<Estilo> {
  return pedir(
    `/v1/importaciones/${id}/estilo/${ranura}${enCategoria(categoria)}`,
    { method: "DELETE", token: conToken(id) },
  );
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
