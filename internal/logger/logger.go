// Package logger builds the application's structured (zap) logger from config.
// It mirrors the pattern used across our services: one configured *zap.Logger,
// installed as the global logger so both injected loggers and zap.L() calls in
// decorators share the same sink and formatting.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a *zap.Logger from a level ("debug"|"info"|"warn"|"error") and a
// format ("json"|"text"). Unknown levels default to info; any non-"json" format
// uses the human-friendly development encoder.
func New(level, format string) (*zap.Logger, error) {
	var lvl zapcore.Level
	switch level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return cfg.Build()
}
