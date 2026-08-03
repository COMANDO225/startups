package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App      App
	Server   Server
	Database Database
	Engine   Engine
	Logger   Logger
	Crypto   Crypto
	Webhook  Webhook
}

type Webhook struct {
	Timeout time.Duration
}

type App struct {
	Name        string
	Environment string
	Version     string
}

type Server struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	BodyLimit    int
	TrustProxy   bool
	ProxyHeader  string
}

type Database struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

type Engine struct {
	URL     string
	Timeout time.Duration
}

type Logger struct {
	Level  string
	Format string
}

// MasterKey cifra los certificados digitales de cada tenant en reposo (AES-256-GCM).
type Crypto struct {
	MasterKey string
}

const claveDesarrollo = "dev-key-no-usar-en-produccion321"

// Espejo de domain.TimeoutProcesando. Se duplica aqui para que config no
// dependa de un feature; el test de coherencia vive en el paquete domain.
const timeoutProcesando = 5 * time.Minute

// JobTimeout debe superar a ENGINE_TIMEOUT: River cancela el contexto del job
// al cumplirse, y un job cancelado no alcanza a reprogramarse ordenadamente.
const JobTimeout = 3 * time.Minute

func (c Config) IsProduction() bool { return c.App.Environment == "production" }

func Load() (*Config, error) {
	cfg := &Config{
		App: App{
			Name:        env("APP_NAME", "facturacion-service"),
			Environment: env("APP_ENV", "development"),
			Version:     env("APP_VERSION", "0.1.0"),
		},
		Server: Server{
			Port:         envInt("SERVER_PORT", 8080),
			ReadTimeout:  envDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  envDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			BodyLimit:    envInt("SERVER_BODY_LIMIT", 4*1024*1024),
			TrustProxy:   envBool("SERVER_TRUST_PROXY", false),
			ProxyHeader:  env("SERVER_PROXY_HEADER", "X-Forwarded-For"),
		},
		Database: Database{
			DSN:         env("DATABASE_URL", ""),
			MaxConns:    int32(envInt("DATABASE_MAX_CONNS", 25)),
			MinConns:    int32(envInt("DATABASE_MIN_CONNS", 2)),
			MaxLifetime: envDuration("DATABASE_MAX_LIFETIME", time.Hour),
			MaxIdleTime: envDuration("DATABASE_MAX_IDLE_TIME", 30*time.Minute),
		},
		Engine: Engine{
			URL:     env("ENGINE_URL", "http://engine:8000"),
			Timeout: envDuration("ENGINE_TIMEOUT", 60*time.Second),
		},
		Logger: Logger{
			Level:  env("LOG_LEVEL", "info"),
			Format: env("LOG_FORMAT", "json"),
		},
		Crypto: Crypto{
			MasterKey: env("CRYPTO_MASTER_KEY", claveDesarrollo),
		},
		Webhook: Webhook{
			Timeout: envDuration("WEBHOOK_TIMEOUT", 10*time.Second),
		},
	}

	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("DATABASE_URL es obligatorio")
	}
	if len(c.Crypto.MasterKey) != 32 {
		return fmt.Errorf("CRYPTO_MASTER_KEY debe tener exactamente 32 bytes, tiene %d", len(c.Crypto.MasterKey))
	}
	if c.IsProduction() && c.Crypto.MasterKey == claveDesarrollo {
		return fmt.Errorf("CRYPTO_MASTER_KEY no puede ser la clave de desarrollo en produccion")
	}

	// Si el timeout del motor superara al de rescate, otro worker retomaria un
	// comprobante que todavia se esta procesando y se enviaria dos veces.
	if c.Engine.Timeout >= timeoutProcesando {
		return fmt.Errorf("ENGINE_TIMEOUT (%s) debe ser menor que el timeout de rescate (%s)",
			c.Engine.Timeout, timeoutProcesando)
	}

	if c.Engine.Timeout >= JobTimeout {
		return fmt.Errorf("ENGINE_TIMEOUT (%s) debe ser menor que JobTimeout (%s): River cancelaria el job antes de que el motor responda",
			c.Engine.Timeout, JobTimeout)
	}

	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return fallback
}
