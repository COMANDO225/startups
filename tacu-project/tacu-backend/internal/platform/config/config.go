// Package config carga la configuracion en capas: valores por defecto, luego el
// YAML, luego el entorno.
//
// Una variable presente pero ilegible MATA el arranque. No cae al default en
// silencio: la diferencia entre "no lo configuraste" y "lo configuraste mal" es
// la diferencia entre un default razonable y un bug de produccion que nadie ve.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	IA       IA       `koanf:"ia"`
	BD       BD       `koanf:"bd"`
	Servidor Servidor `koanf:"servidor"`
	Almacen  Almacen  `koanf:"almacen"`
}

type BD struct {
	// DSN nunca va en el YAML: es una credencial. Entra por TACU_BD_DSN.
	DSN           string `koanf:"dsn"`
	MaxConexiones int32  `koanf:"max_conexiones"`
}

type Servidor struct {
	Puerto int `koanf:"puerto"`

	// TamanoMaxSubidaMB topea el multipart. Cuatro fotos de carta a 10 MB cada
	// una son 40; sin tope, una peticion puede agotar la memoria del proceso.
	TamanoMaxSubidaMB int `koanf:"tamano_max_subida_mb"`

	// OrigenesPermitidos son los origenes del navegador que pueden llamar a la
	// API. En desarrollo el frontend vive en otro puerto, asi que sin esto el
	// navegador bloquea TODA llamada — y el sintoma es un "Failed to fetch" sin
	// codigo de estado, que parece que el backend esta caido cuando en realidad
	// responde perfecto a curl.
	//
	// Es una lista explicita y NO un comodin: con "*" el navegador se niega a
	// mandar la cabecera Authorization, que es justo la que lleva el token.
	OrigenesPermitidos []string `koanf:"origenes_permitidos"`

	// LeerCartaSincrono hace que POST /v1/importaciones espere a que la carta
	// se lea, unos 10 s. Es feo pero desbloquea al frontend dos etapas antes de
	// que exista la cola, y cambiarlo despues no altera ni un campo del JSON.
	LeerCartaSincrono bool `koanf:"leer_carta_sincrono"`
}

type Almacen struct {
	// Tipo elige la implementacion: "disco" para desarrollo y tests, "r2" para
	// lo demas.
	Tipo string `koanf:"tipo"`

	// Raiz es donde viven las imagenes en disco.
	Raiz string `koanf:"raiz"`
	// Base es el prefijo con el que las sirve nuestra API.
	Base string `koanf:"base"`

	R2 R2 `koanf:"r2"`
}

type R2 struct {
	// Cuenta, ClaveID y Secreto son credenciales: NO van en el YAML, entran por
	// entorno como el DSN y las claves de IA.
	Cuenta  string `koanf:"cuenta"`
	ClaveID string `koanf:"clave_id"`
	Secreto string `koanf:"secreto"`

	// Dos buckets porque en R2 el acceso publico es POR BUCKET: no hay carpeta
	// publica dentro de uno privado.
	BucketPublico string `koanf:"bucket_publico"`
	BucketPrivado string `koanf:"bucket_privado"`

	// DominioPublico vacio = las fotos publicas tambien salen por nuestra API.
	// Es lo que permite ver las imagenes antes de tener el subdominio atado.
	DominioPublico string `koanf:"dominio_publico"`
}

// UsaR2 dice si hay que montar el bucket en vez del disco.
func (a Almacen) UsaR2() bool { return a.Tipo == "r2" }

type IA struct {
	Proveedores map[string]Proveedor `koanf:"proveedores"`
	Tareas      map[string][]string  `koanf:"tareas"`
	Precios     map[string]Precio    `koanf:"precios"`

	PresupuestoPorImportacionUSD float64 `koanf:"presupuesto_por_importacion_usd"`
	MaxConcurrenciaPorProveedor  int     `koanf:"max_concurrencia_por_proveedor"`
}

type Proveedor struct {
	APIKey string `koanf:"api_key"`
}

// Precio es un ESTIMADO LOCAL, no la verdad. No se le envia al proveedor ni
// limita lo que cobra: solo multiplica los tokens que el proveedor reporta.
//
// Sirve para lo unico que la factura mensual no dice: cuanto costo importar la
// carta de UN restaurante. Esa cifra decide si el plan da margen.
//
// VigenteHasta existe porque una tarifa promocional que vence en silencio
// convierte todos los calculos en mentira sin que nadie se entere. Con la fecha
// declarada, un test revienta ese dia y obliga a revisarla.
type Precio struct {
	EntradaPorMillon float64 `koanf:"entrada_por_millon"`
	SalidaPorMillon  float64 `koanf:"salida_por_millon"`
	PorImagen        float64 `koanf:"por_imagen"`
	VigenteHasta     string  `koanf:"vigente_hasta"` // AAAA-MM-DD, opcional
}

// clavesPorEnv mapea la variable de entorno "corta" y comoda a su ruta en el
// YAML. Existen porque GEMINI_API_KEY es lo que todo el mundo escribe en un
// .env, y nadie quiere teclear TACU_IA__PROVEEDORES__GEMINI__API_KEY.
var clavesPorEnv = map[string]string{
	"GEMINI_API_KEY":    "ia.proveedores.gemini.api_key",
	"OPENAI_API_KEY":    "ia.proveedores.openai.api_key",
	"TACU_BD_DSN":       "bd.dsn",
	"TACU_PUERTO":       "servidor.puerto",
	"TACU_ALMACEN":      "almacen.raiz",
	"TACU_ALMACEN_TIPO": "almacen.tipo",

	// Las de R2. Las dos ultimas son credenciales y por eso entran por aqui y
	// no por el YAML, igual que TACU_BD_DSN.
	"R2_CUENTA":            "almacen.r2.cuenta",
	"R2_BUCKET_PUBLICO":    "almacen.r2.bucket_publico",
	"R2_BUCKET_PRIVADO":    "almacen.r2.bucket_privado",
	"R2_DOMINIO_PUBLICO":   "almacen.r2.dominio_publico",
	"R2_ACCESS_KEY_ID":     "almacen.r2.clave_id",
	"R2_SECRET_ACCESS_KEY": "almacen.r2.secreto",
}

// Cargar lee el YAML y lo superpone con el entorno.
func Cargar(ruta string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(ruta), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", ruta, err)
	}

	// El entorno pisa al YAML: es lo que permite cambiar un modelo en produccion
	// sin reconstruir la imagen.
	for env, ruta := range clavesPorEnv {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			if err := k.Set(ruta, v); err != nil {
				return nil, fmt.Errorf("aplicando %s: %w", env, err)
			}
		}
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("interpretando la configuracion: %w", err)
	}

	if err := cfg.validar(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validar acumula TODOS los errores en vez de morir en el primero: arreglar la
// configuracion de a un error por arranque es una perdida de tiempo.
func (c *Config) validar() error {
	var problemas []error

	if len(c.IA.Tareas) == 0 {
		problemas = append(problemas, errors.New("ia.tareas esta vacio"))
	}

	for tarea, cadena := range c.IA.Tareas {
		if len(cadena) == 0 {
			problemas = append(problemas, fmt.Errorf("ia.tareas.%s: cadena vacia", tarea))
			continue
		}
		for _, ref := range cadena {
			prov, _, ok := strings.Cut(ref, "/")
			if !ok || prov == "" {
				problemas = append(problemas,
					fmt.Errorf("ia.tareas.%s: %q no tiene el formato proveedor/modelo", tarea, ref))
				continue
			}
			if _, declarado := c.IA.Proveedores[prov]; !declarado {
				problemas = append(problemas,
					fmt.Errorf("ia.tareas.%s: usa el proveedor %q, que no esta en ia.proveedores", tarea, prov))
			}
			if _, tienePrecio := c.IA.Precios[ref]; !tienePrecio {
				// Sin precio el costo sale 0 y el unit economics queda invisible.
				problemas = append(problemas,
					fmt.Errorf("ia.precios: falta la tarifa de %q (usada en %s)", ref, tarea))
			}
		}
	}

	if c.IA.PresupuestoPorImportacionUSD <= 0 {
		problemas = append(problemas, errors.New("ia.presupuesto_por_importacion_usd debe ser mayor que cero"))
	}
	if c.IA.MaxConcurrenciaPorProveedor < 1 {
		problemas = append(problemas, errors.New("ia.max_concurrencia_por_proveedor debe ser al menos 1"))
	}

	// Con r2 se exigen las seis de una vez y con su nombre de variable: que el
	// arranque falle seis veces seguidas, una por cada campo, es tiempo tirado.
	if c.Almacen.UsaR2() {
		for _, f := range []struct{ env, valor string }{
			{"R2_CUENTA", c.Almacen.R2.Cuenta},
			{"R2_BUCKET_PUBLICO", c.Almacen.R2.BucketPublico},
			{"R2_BUCKET_PRIVADO", c.Almacen.R2.BucketPrivado},
			{"R2_ACCESS_KEY_ID", c.Almacen.R2.ClaveID},
			{"R2_SECRET_ACCESS_KEY", c.Almacen.R2.Secreto},
		} {
			if strings.TrimSpace(f.valor) == "" {
				problemas = append(problemas,
					fmt.Errorf("almacen.tipo es r2 y falta %s", f.env))
			}
		}
	}
	if t := c.Almacen.Tipo; t != "" && t != "disco" && t != "r2" {
		problemas = append(problemas, fmt.Errorf("almacen.tipo %q: solo vale disco o r2", t))
	}

	// El DSN no se valida aqui: los CLIs de laboratorio no tocan la base y
	// exigirselo los rompe. Quien lo necesita es cmd/api, y ahi se comprueba.
	if c.Servidor.Puerto != 0 && (c.Servidor.Puerto < 1 || c.Servidor.Puerto > 65535) {
		problemas = append(problemas, fmt.Errorf("servidor.puerto %d esta fuera de rango", c.Servidor.Puerto))
	}
	if c.Servidor.TamanoMaxSubidaMB < 0 {
		problemas = append(problemas, errors.New("servidor.tamano_max_subida_mb no puede ser negativo"))
	}

	return errors.Join(problemas...)
}
