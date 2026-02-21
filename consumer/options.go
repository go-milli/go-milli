package consumer

import (
	"context"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/codec"
	"github.com/go-milli/go-milli/codec/json"
	"github.com/go-milli/go-milli/logger"
)

type Options struct {
	Broker             broker.Broker
	Codec              codec.Marshaler
	Logger             logger.Logger
	Context            context.Context
	ID                 string
	Name               string
	SubscriberWrappers []SubscriberWrapper
}

type Option func(*Options)

func NewOptions(opts ...Option) Options {
	options := Options{
		Logger: logger.DefaultLogger,
		Codec:  json.Marshaler{},
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

// Context sets the context
func Context(ctx context.Context) Option {
	return func(o *Options) {
		o.Context = ctx
	}
}

// WrapSubscriber adds a subscriber wrapper to a list of options passed into the consumer
func WrapSubscriber(w ...SubscriberWrapper) Option {
	return func(o *Options) {
		o.SubscriberWrappers = append(o.SubscriberWrappers, w...)
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
		// 默认关闭底层的无脑发后不理（AutoAck），强制要求框架进行基于业务结果的“安全确认”
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

// LoggingMiddleware is an example middleware that logs topic names
func LoggingMiddleware(next Handler) Handler {
	return func(ctx context.Context, msg Message) error {
		logger.Infof("[LOG] Processing message from topic: %s", msg.Topic)
		err := next(ctx, msg)
		if err != nil {
			logger.Errorf("[LOG] Error: %v", err)
		}
		return err
	}
}

type contextAckKey struct{}

// ContextAckWrapper is a middleware that injects the message's Ack closure into the context.
// This allows downstream handlers (especially generic handlers) to manually ack the message.
// Use this middleware ONLY when AutoAck is disabled.
func ContextAckWrapper(next Handler) Handler {
	return func(ctx context.Context, msg Message) error {
		if msg.Ack != nil {
			ctx = context.WithValue(ctx, contextAckKey{}, msg.Ack)
		}
		return next(ctx, msg)
	}
}

// Ack manually executes the message acknowledgement if ContextAckWrapper was used.
func Ack(ctx context.Context) error {
	if ack, ok := ctx.Value(contextAckKey{}).(func() error); ok && ack != nil {
		return ack()
	}
	return nil
}
