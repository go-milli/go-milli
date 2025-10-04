package consumer

import (
	"context"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/logger"
)

type Options struct {
	Broker  broker.Broker
	Logger  logger.Logger
	Context context.Context
	ID      string
	Name    string
}

type Option func(*Options)

func NewOptions(opts ...Option) Options {
	options := Options{
		Logger: logger.DefaultLogger,
	}

	for _, o := range opts {
		o(&options)
	}

	if options.Broker == nil {
		options.Broker = broker.DefaultBroker
	}

	if len(options.ID) == 0 {
		options.ID = DefaultID
	}

	if len(options.Name) == 0 {
		options.Name = DefaultName
	}

	return options
}

// ID sets the consumer ID.
func ID(id string) Option {
	return func(o *Options) {
		o.ID = id
	}
}

// Name sets the consumer name.
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

// Logger sets the logger
func Logger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// Context sets the context
func Context(ctx context.Context) Option {
	return func(o *Options) {
		o.Context = ctx
	}
}

// SubscriberOptions for subscription
type SubscriberOptions struct {
	Context context.Context
	Queue   string
	AutoAck bool
}

type SubscriberOption func(*SubscriberOptions)

func NewSubscriberOptions(opts ...SubscriberOption) SubscriberOptions {
	options := SubscriberOptions{
		AutoAck: true,
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&options)
	}

	return options
}

// SubscriberQueue sets the queue name
func SubscriberQueue(name string) SubscriberOption {
	return func(o *SubscriberOptions) {
		o.Queue = name
	}
}

// DisableAutoAck disables auto ack
func DisableAutoAck() SubscriberOption {
	return func(o *SubscriberOptions) {
		o.AutoAck = false
	}
}

// SubscriberContext sets the context
func SubscriberContext(ctx context.Context) SubscriberOption {
	return func(o *SubscriberOptions) {
		o.Context = ctx
	}
}

// func loggingMiddleware(next Handler) Handler {
// 	return func(ctx context.Context, msg *Message) error {
// 		fmt.Printf("[LOG] Processing message from topic: %s\n", msg.Topic)
// 		err := next(ctx, msg)
// 		if err != nil {
// 			fmt.Printf("[LOG] Error: %v\n", err)
// 		}
// 		return err
// 	}
// }

// func tracingMiddleware(next Handler) Handler {
// 	return func(ctx context.Context, msg *Message) error {
// 		traceID := msg.Header["Trace-Id"]
// 		fmt.Printf("[TRACE] %s - Start\n", traceID)
// 		err := next(ctx, msg)
// 		fmt.Printf("[TRACE] %s - End\n", traceID)
// 		return err
// 	}
// }
