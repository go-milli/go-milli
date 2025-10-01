// Package zap provides a zap-backed logger implementation for the framework logger.
package zap

import (
	"context"
	"fmt"
	"os"
	"sync"

	stdzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"worker/logger"
)

// zaplog mirrors the plugins-style implementation: configurable via Options.Context.
type zaplog struct {
	cfg  stdzap.Config
	zap  *stdzap.Logger
	opts logger.Options
	sync.RWMutex
	fields map[string]interface{}
}

// Context keys to pass advanced zap configuration through logger.Options.Context.
type (
	configKey        struct{}
	encoderConfigKey struct{}
	optionsKey       struct{}
	namespaceKey     struct{}
	loggerKey        struct{}
)

func (l *zaplog) Init(opts ...logger.Option) error {
	for _, o := range opts {
		o(&l.opts)
	}

	// base config
	cfg := stdzap.NewProductionConfig()

	// extract overrides from Options.Context if provided
	ctx := l.opts.Context
	if ctx != nil {
		if v, ok := ctx.Value(configKey{}).(stdzap.Config); ok {
			cfg = v
		}
		if v, ok := ctx.Value(encoderConfigKey{}).(zapcore.EncoderConfig); ok {
			cfg.EncoderConfig = v
		}
	}

	cfg.Level = stdzap.NewAtomicLevel()
	if l.opts.Level != logger.InfoLevel {
		cfg.Level.SetLevel(loggerToZapLevel(l.opts.Level))
	}

	var zopts []stdzap.Option
	if l.opts.CallerSkipCount > 0 {
		zopts = append(zopts, stdzap.AddCallerSkip(l.opts.CallerSkipCount))
	}
	if ctx != nil {
		if more, ok := ctx.Value(optionsKey{}).([]stdzap.Option); ok {
			zopts = append(zopts, more...)
		}
	}

	z, err := cfg.Build(zopts...)
	if err != nil {
		return err
	}

	// seed fields
	if l.opts.Fields != nil {
		fields := make([]stdzap.Field, 0, len(l.opts.Fields))
		for k, v := range l.opts.Fields {
			fields = append(fields, stdzap.Any(k, v))
		}
		z = z.With(fields...)
	}

	// namespace and injected logger
	if ctx != nil {
		if ns, ok := ctx.Value(namespaceKey{}).(string); ok && ns != "" {
			z = z.With(stdzap.Namespace(ns))
		}
		if injected, ok := ctx.Value(loggerKey{}).(*stdzap.Logger); ok && injected != nil {
			l.zap = injected
		}
	}

	l.cfg = cfg
	if l.zap == nil {
		l.zap = z
	}
	l.fields = make(map[string]interface{})
	return nil
}

func (l *zaplog) Fields(fields map[string]interface{}) logger.Logger {
	l.Lock()
	nfields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		nfields[k] = v
	}
	l.Unlock()
	for k, v := range fields {
		nfields[k] = v
	}

	data := make([]stdzap.Field, 0, len(fields))
	for k, v := range fields {
		data = append(data, stdzap.Any(k, v))
	}

	nl := &zaplog{cfg: l.cfg, zap: l.zap.With(data...), opts: l.opts, fields: make(map[string]interface{})}
	return nl
}

func (l *zaplog) Log(level logger.Level, args ...interface{}) {
	l.RLock()
	data := make([]stdzap.Field, 0, len(l.fields))
	for k, v := range l.fields {
		data = append(data, stdzap.Any(k, v))
	}
	l.RUnlock()

	msg := fmt.Sprint(args...)
	switch loggerToZapLevel(level) {
	case stdzap.DebugLevel:
		l.zap.Debug(msg, data...)
	case stdzap.InfoLevel:
		l.zap.Info(msg, data...)
	case stdzap.WarnLevel:
		l.zap.Warn(msg, data...)
	case stdzap.ErrorLevel:
		l.zap.Error(msg, data...)
	case stdzap.FatalLevel:
		l.zap.Fatal(msg, data...)
	}
}

func (l *zaplog) Logf(level logger.Level, format string, args ...interface{}) {
	l.RLock()
	data := make([]stdzap.Field, 0, len(l.fields))
	for k, v := range l.fields {
		data = append(data, stdzap.Any(k, v))
	}
	l.RUnlock()

	msg := fmt.Sprintf(format, args...)
	switch loggerToZapLevel(level) {
	case stdzap.DebugLevel:
		l.zap.Debug(msg, data...)
	case stdzap.InfoLevel:
		l.zap.Info(msg, data...)
	case stdzap.WarnLevel:
		l.zap.Warn(msg, data...)
	case stdzap.ErrorLevel:
		l.zap.Error(msg, data...)
	case stdzap.FatalLevel:
		l.zap.Fatal(msg, data...)
	}
}

func (l *zaplog) String() string          { return "zap" }
func (l *zaplog) Options() logger.Options { return l.opts }

// NewLogger builds a zap-backed logger using logger.Options, similar to plugins style.
func NewLogger(opts ...logger.Option) (logger.Logger, error) {
	options := logger.Options{Level: logger.InfoLevel, Fields: map[string]interface{}{}, Out: os.Stderr, CallerSkipCount: 2, Context: context.Background()}
	l := &zaplog{opts: options}
	if err := l.Init(opts...); err != nil {
		return nil, err
	}
	return l, nil
}

func loggerToZapLevel(level logger.Level) zapcore.Level {
	switch level {
	case logger.TraceLevel, logger.DebugLevel:
		return zapcore.DebugLevel
	case logger.InfoLevel:
		return zapcore.InfoLevel
	case logger.WarnLevel:
		return zapcore.WarnLevel
	case logger.ErrorLevel:
		return zapcore.ErrorLevel
	case logger.FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
