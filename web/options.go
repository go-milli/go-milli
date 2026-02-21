package web

import (
	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/publisher"
)

// Options for the web service.
type Options struct {
	Name              string
	Address           string
	Mode              string // gin mode: "debug" / "release" / "test"
	Broker            broker.Broker
	PublisherWrappers []publisher.Wrapper
}

type Option func(*Options)

func newOptions(opts ...Option) Options {
	o := Options{
		Name:    "milli.web",
		Address: ":8080",
		Mode:    "release",
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Name sets the service name.
func Name(n string) Option {
	return func(o *Options) { o.Name = n }
}

// Address sets the listen address. Default ":8080".
func Address(addr string) Option {
	return func(o *Options) { o.Address = addr }
}

// Mode sets the gin mode ("debug" / "release" / "test").
func Mode(m string) Option {
	return func(o *Options) { o.Mode = m }
}

// Broker sets the broker for optional Publisher support.
func Broker(b broker.Broker) Option {
	return func(o *Options) { o.Broker = b }
}

// WrapPublisher adds publisher wrappers (e.g. OTel tracing).
func WrapPublisher(w ...publisher.Wrapper) Option {
	return func(o *Options) {
		o.PublisherWrappers = append(o.PublisherWrappers, w...)
	}
}
