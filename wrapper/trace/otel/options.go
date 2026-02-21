package otel

import (
	"go.opentelemetry.io/otel/trace"
)

// Options holds configuration for the tracing wrapper
type Options struct {
	TraceProvider trace.TracerProvider
}

// Option configures the Options structure
type Option func(*Options)

// NewOptions creates a new set of options
func NewOptions(opts ...Option) *Options {
	options := &Options{}

	for _, o := range opts {
		o(options)
	}

	return options
}

// WithTraceProvider sets the trace provider
func WithTraceProvider(tp trace.TracerProvider) Option {
	return func(o *Options) {
		o.TraceProvider = tp
	}
}
