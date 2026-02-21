// Package consumer provides the orchestration layer on top of a broker.
package consumer

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/aevumio/go-milli/logger"
	"github.com/google/uuid"
)

// Consumer interface
type Consumer interface {
	// Retrieve the options
	Options() Options
	// Subscribe registers a handler for a topic
	Subscribe(topic string, handler Handler, opts ...SubscriberOption) error

	// Start connects and begins consuming
	Start() error

	// Stop unsubscribes and disconnects
	Stop() error
	// Consumer implementation
	String() string
}

// Handler processes messages
type Handler func(ctx context.Context, msg Message) error

// SubscriberWrapper is a middleware interface for message processing
type SubscriberWrapper func(Handler) Handler

// Message wraps broker message
type Message struct {
	Topic  string
	Header map[string]string
	Body   []byte
	// Ack is a closure that allows the user to manually acknowledge the message
	Ack func() error
}

type Subscriber interface {
	Topic() string
	Handler() Handler
	Options() SubscriberOptions
}

var (
	DefaultID                = uuid.New().String()
	DefaultName              = "go-milli-consumer"
	DefaultConsumer Consumer = NewDefaultConsumer()
)

// DefaultOptions returns the default options for the consumer.
func DefaultOptions() Options {
	return DefaultConsumer.Options()
}

// Subscribe subscribes to a topic with the default consumer.
func Subscribe(topic string, handler Handler, opts ...SubscriberOption) error {
	return DefaultConsumer.Subscribe(topic, handler, opts...)
}

// Run starts the default consumer and waits for a kill
// signal before exiting.
func Run() error {
	if err := Start(); err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)
	shutdownSignals := []os.Signal{
		syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL,
	}
	signal.Notify(ch, shutdownSignals...)
	DefaultConsumer.Options().Logger.Logf(logger.InfoLevel, "Received signal %s", <-ch)

	return Stop()
}

// Start starts the default consumer.
func Start() error {
	config := DefaultConsumer.Options()
	config.Logger.Logf(logger.InfoLevel, "Starting server %s id %s", config.Name, config.ID)
	return DefaultConsumer.Start()
}

// Stop stops the default consumer.
func Stop() error {
	defaultOptions := DefaultConsumer.Options()
	defaultOptions.Logger.Logf(logger.InfoLevel, "Stopping server %s id %s", defaultOptions.Name, defaultOptions.ID)
	return DefaultConsumer.Stop()
}

// String returns the default consumer name.
func String() string {
	return DefaultConsumer.String()
}
