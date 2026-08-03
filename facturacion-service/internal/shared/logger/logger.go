package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger with structured logging helpers.
type Logger struct {
	*zap.SugaredLogger
	base *zap.Logger
}

// New creates a Logger based on environment configuration.
func New(level, format string) (*Logger, error) {
	var cfg zap.Config

	switch format {
	case "json":
		cfg = zap.NewProductionConfig()
	default:
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("parsing log level %q: %w", level, err)
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	base, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return &Logger{
		SugaredLogger: base.Sugar(),
		base:          base,
	}, nil
}

// WithRequestID returns a logger with the request_id field.
func (l *Logger) WithRequestID(requestID string) *Logger {
	child := l.base.With(zap.String("request_id", requestID))
	return &Logger{
		SugaredLogger: child.Sugar(),
		base:          child,
	}
}

// WithFields returns a logger with additional structured fields.
func (l *Logger) WithFields(fields ...any) *Logger {
	child := l.SugaredLogger.With(fields...)
	return &Logger{
		SugaredLogger: child,
		base:          l.base,
	}
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.base.Sync()
}
