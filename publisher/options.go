package publisher

import (
	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/codec"
	"github.com/aevumio/go-milli/codec/json"
	"github.com/aevumio/go-milli/logger"
)

type Options struct {
	Name     string
	Broker   broker.Broker
	Codec    codec.Marshaler
	Logger   logger.Logger
	Wrappers []Wrapper
}

type Option func(*Options)

// NewOptions creates a new set of options
func NewOptions(opts ...Option) Options {
	options := Options{
		Name:   "go-milli.publisher",
		Logger: logger.DefaultLogger,
		Codec:  json.Marshaler{},
	}

	for _, o := range opts {
		o(&options)
	}

	if options.Broker == nil {
		options.Broker = broker.DefaultBroker
	}

	return options
}

// Name sets the name
func Name(n string) Option {
	return func(o *Options) {
		o.Name = n
	}
}

// Broker sets the broker
func Broker(b broker.Broker) Option {
	return func(o *Options) {
		o.Broker = b
	}
}

// Codec sets the codec
func Codec(c codec.Marshaler) Option {
	return func(o *Options) {
		o.Codec = c
	}
}

// Logger sets the logger
func Logger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// WrapPublisher adds a wrapper to the publisher options
func WrapPublisher(w ...Wrapper) Option {
	return func(o *Options) {
		o.Wrappers = append(o.Wrappers, w...)
	}
}

// PublishOptions for individual publish calls
type PublishOptions struct {
	// Add other options here if necessary
}

type PublishOption func(*PublishOptions)

// NewPublishOptions creates publish options
func NewPublishOptions(opts ...PublishOption) PublishOptions {
	options := PublishOptions{}

	for _, o := range opts {
		o(&options)
	}

	return options
}
