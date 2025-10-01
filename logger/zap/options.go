package zap

import (
	stdzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"worker/logger"
)

// WithConfig injects a zap.Config via logger.Options.Context.
func WithConfig(cfg stdzap.Config) logger.Option { return logger.SetOption(configKey{}, cfg) }

// WithEncoderConfig injects a zapcore.EncoderConfig via logger.Options.Context.
func WithEncoderConfig(enc zapcore.EncoderConfig) logger.Option {
	return logger.SetOption(encoderConfigKey{}, enc)
}

// WithOptions appends extra zap options via logger.Options.Context.
func WithOptions(opts ...stdzap.Option) logger.Option { return logger.SetOption(optionsKey{}, opts) }

// WithNamespace sets a zap namespace for subsequent fields.
func WithNamespace(ns string) logger.Option { return logger.SetOption(namespaceKey{}, ns) }

// WithZapLogger injects an existing *zap.Logger to be used directly.
func WithZapLogger(z *stdzap.Logger) logger.Option { return logger.SetOption(loggerKey{}, z) }
