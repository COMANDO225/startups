// Espejo exacto del JSON del backend. Si un campo no esta aqui, el backend no lo manda.

/** 'nueva' es el restaurante creado y todavia sin hojas de carta. */
export type EstadoImportacion =
  "nueva" | "leyendo" | "lista" | "publicada" | "fallida";

export type EstadoFoto =
  "vacia" | "pendiente" | "generando" | "lista" | "error" | "sin_presupuesto";

export type OrigenFoto = "ia" | "propia";

export type Foto = {
  estado: EstadoFoto;
  origen?: OrigenFoto;

  /** La grande, 1280. Para el detalle y para volver a pasarla por el modelo. */
  url?: string;
  /** 640, para las tarjetas del editor. */
  url_media?: string;
  /** 320, para el catalogo publico, que la pinta a 80 px. */
  url_pequena?: string;
};

export type Precio = {
  // OPCIONALES de verdad: en Go llevan `omitempty`, asi que cuando estan vacios
  // la clave NO viaja en el JSON. Declararlos obligatorios era una mentira de
  // tipo con consecuencia: `value={undefined}` convierte el input de la etiqueta
  // en no-controlado, y React avisa en cuanto el dueno escribe — justo en la
  // pantalla que existe para rellenar esa etiqueta.
  etiqueta?: string;
  manuscrito?: boolean;

  soles: string;
  centimos: number;
  impreso: string;
};

export type Revisar = {
  motivo: string;
  explicacion: string;
  // true impide publicar (rojo); false solo pide confirmar (ambar suave). No mezclar.
  bloquea: boolean;
};

export type Plato = {
  id: string;
  nombre: string;
  descripcion?: string;
  precios: Precio[];
  desde: string;
  revisar?: Revisar;
  foto?: Foto;

  /** Lo que el dueno escribio para corregir la foto de ESTE plato. */
  foto_ajuste?: string;

  /** URLs de las fotos de ejemplo del plato. */
  foto_referencias?: string[];

  /** La ultima lectura ya no lo trajo. */
  ausente?: boolean;

  /** Clave de la hoja de la que salio. Vacia si no se sabe. */
  hoja?: string;
};

export type TipoRestaurante = {
  clave: string;
  nombre: string;
  descripcion: string;
};

/**
 * Una de las dos cosas que el dueno decide de sus fotos.
 *
 * `imagen` viene ya resuelta del backend —su foto si la hay, si no el dibujo—
 * porque la precedencia es una regla del dominio: repetirla aqui seria tener dos
 * versiones de ella esperando a divergir. Vacia = la de por defecto, que viaja
 * con esta app y no cuesta nada.
 */
export type Ranura = {
  texto: string;
  imagen: string;
  /** La imagen es una foto suya, no un dibujo. */
  propia: boolean;
  /** Toco algo aqui, asi que hay a donde volver. */
  tocada: boolean;
};

/** El estilo de un ambito: la base general o la de una categoria. */
export type Estilo = {
  categoria: string;
  vajilla: Ranura;
  fondo: Ranura;
};

/** Las dos ranuras, y no hay mas. */
export type CualRanura = "vajilla" | "fondo";

export type ConteoDeTipo = {
  clave: string;
  nombre: string;
  platos: number;
};

/**
 * Que le pasa a la carta con los tipos elegidos. Lo calcula el backend con las
 * MISMAS pistas que deciden la foto, asi que esto es literalmente lo que va a
 * ocurrir al generar.
 */
export type Reparto = {
  total: number;
  por_tipo: ConteoDeTipo[];

  /** Platos que no se parecen a ninguno: salen con la guarnicion del primero. */
  sin_pistas: number;

  /** Tipos NO elegidos que serian el mejor tipo de varios platos. */
  faltan: ConteoDeTipo[];
};

export type TiposDeNegocio = {
  /** Los elegidos, en orden: el primero es el principal. */
  elegidos: string[];
  catalogo: TipoRestaurante[];
  reparto: Reparto;
};

export type Referencias = { urls: string[]; claves: string[] };

export type Publicada = { slug: string; ruta: string };

export type PlatoEditado = {
  plato: Plato;
  marcas: { revisar: number; confirmar: number };
};

export type Categoria = {
  nombre: string;
  platos: Plato[];
};

/** Una hoja de la carta de papel. La clave la identifica; la url la pinta. */
export type Pagina = {
  clave: string;
  url: string;
};

export type Restaurante = {
  nombre: string;
  slug?: string;
};

export type Gasto = {
  gastado_usd: number;
  presupuesto_usd: number;
  /** Viene del backend, no se copia aqui: cambia con el modelo de config.yaml. */
  por_foto_usd: number;
};

export type Importacion = {
  /** En que va la lectura, 0..6. Solo significa algo con estado "leyendo". */
  etapa: number;
  id: string;
  estado: EstadoImportacion;
  restaurante: Restaurante;
  error?: string; // solo con estado "fallida"
  marcas: { revisar: number; confirmar: number };
  gasto: Gasto;
  categorias: Categoria[]; // [] mientras estado === "leyendo": senal para esqueletos

  paginas: Pagina[];

  /** Hojas que se estan leyendo AHORA. No esconde la carta: solo avisa de que vienen mas platos. */

  puede_publicarse: boolean;
};

export type ImportacionCreada = {
  id: string;
  estado: EstadoImportacion;
  token: string; // viaja UNA sola vez, en esta respuesta
  restaurante: Restaurante;
};

export type EstadoFotos = {
  etapa: number;
  estado: EstadoImportacion;
  fotos: Record<string, Foto>; // mapa por id de plato, para que cada tarjeta lea solo lo suyo
  pendientes: number;
  gasto: Gasto;
};

export type ErrorApi = {
  error: string;
  detalle?: string;
};
