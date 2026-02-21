package milli

import (
	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/codec"
	"github.com/aevumio/go-milli/codec/json"
	"github.com/aevumio/go-milli/consumer"
	"github.com/aevumio/go-milli/publisher"
)

type Options struct {
	Name               string
	Broker             broker.Broker
	Codec              codec.Marshaler
	Consumer           consumer.Consumer
	Publisher          publisher.Publisher
	SubscriberWrappers []consumer.SubscriberWrapper
	PublisherWrappers  []publisher.Wrapper
}

type Option func(*Options)

// Name sets the service name
func Name(n string) Option {
	return func(o *Options) {
		o.Name = n
	}
}

// Broker sets the broker for the service setup
func Broker(b broker.Broker) Option {
	return func(o *Options) {
		o.Broker = b
	}
}

// Codec sets the codec for the service setup
func Codec(c codec.Marshaler) Option {
	return func(o *Options) {
		o.Codec = c
	}
}

// Consumer sets the consumer for the service setup
func Consumer(c consumer.Consumer) Option {
	return func(o *Options) {
		o.Consumer = c
	}
}

// Publisher sets the publisher for the service setup
func Publisher(p publisher.Publisher) Option {
	return func(o *Options) {
		o.Publisher = p
	}
}

// WrapSubscriber adds a subscriber wrapper to the service setup
func WrapSubscriber(w ...consumer.SubscriberWrapper) Option {
	return func(o *Options) {
		o.SubscriberWrappers = append(o.SubscriberWrappers, w...)
	}
}

// WrapPublisher adds a publisher wrapper to the service setup
func WrapPublisher(w ...publisher.Wrapper) Option {
	return func(o *Options) {
		o.PublisherWrappers = append(o.PublisherWrappers, w...)
	}
}

func newOptions(opts ...Option) Options {
	options := Options{
		Name:   "milli.service",
		Broker: broker.DefaultBroker,
		Codec:  json.Marshaler{},
	}

	for _, o := range opts {
		o(&options)
	}

	return options
}
