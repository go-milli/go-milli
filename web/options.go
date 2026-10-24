package web

import (
	"time"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/publisher"
)

// Options for the web service.
type Options struct {
	Name              string
	Address           string
	Mode              string // gin mode: "debug" / "release" / "test"
	Broker            broker.Broker
	PublisherWrappers []publisher.Wrapper
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type Option func(*Options)

func newOptions(opts ...Option) Options {
	o := Options{
		Name:              "milli.web",
		Address:           ":8080",
		Mode:              "release",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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

// ReadHeaderTimeout limits how long a client may take to send HTTP headers.
// Default: 5 seconds. Zero disables the deadline (subject to ReadTimeout).
func ReadHeaderTimeout(d time.Duration) Option {
	return func(o *Options) { o.ReadHeaderTimeout = d }
}

// ReadTimeout limits reading the entire request, including its body.
// Default: 30 seconds. Adjust for uploads; zero disables the deadline.
func ReadTimeout(d time.Duration) Option {
	return func(o *Options) { o.ReadTimeout = d }
}

// WriteTimeout limits writing a response. Default: 30 seconds.
// Streaming endpoints may need a longer timeout or zero to disable it.
func WriteTimeout(d time.Duration) Option {
	return func(o *Options) { o.WriteTimeout = d }
}

// IdleTimeout limits idle keep-alive connections. Default: 60 seconds.
// Zero falls back to ReadTimeout, following net/http semantics.
func IdleTimeout(d time.Duration) Option {
	return func(o *Options) { o.IdleTimeout = d }
}
