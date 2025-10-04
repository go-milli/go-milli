package consumer

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/logger"
	"github.com/go-milli/go-milli/metadata.go"
)

// defaultConsumer implementation
type defaultConsumer struct {
	opts Options

	// subscribers maps subscriber -> broker.Subscriber
	subscribers map[*subscriber]broker.Subscriber

	sync.RWMutex
	// marks the consumer as started
	started bool
}

// NewDefaultConsumer creates a new consumer
func NewDefaultConsumer(opts ...Option) Consumer {
	return &defaultConsumer{
		opts:        NewOptions(opts...),
		subscribers: make(map[*subscriber]broker.Subscriber),
	}
}

// Subscribe registers a handler for a topic
func (c *defaultConsumer) Subscribe(topic string, handler Handler, opts ...SubscriberOption) error {
	c.Lock()
	defer c.Unlock()

	// Create subscription options
	options := SubscriberOptions{
		AutoAck: true,
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&options)
	}

	// Create subscription
	sub := &subscriber{
		topic:   topic,
		handler: handler,
		opts:    options,
	}

	// Check if already subscribed
	for existing := range c.subscribers {
		if existing.topic == topic && fmt.Sprintf("%p", existing.handler) == fmt.Sprintf("%p", handler) {
			return fmt.Errorf("already subscribed to topic %s with this handler", topic)
		}
	}

	// Save subscription (nil means not yet subscribed to broker)
	c.subscribers[sub] = nil

	c.opts.Logger.Logf(logger.DebugLevel, "Registered subscription for topic: %s", topic)

	return nil
}

// Start connects to broker and starts all subscriptions
func (c *defaultConsumer) Start() error {
	c.Lock()
	defer c.Unlock()

	if c.started {
		return nil
	}

	// Connect to broker
	if err := c.opts.Broker.Connect(); err != nil {
		return fmt.Errorf("failed to connect broker: %w", err)
	}

	c.opts.Logger.Logf(logger.InfoLevel, "Broker connected: %s", c.opts.Broker.Address())

	// Subscribe to all registered topics
	for sub := range c.subscribers {
		if c.subscribers[sub] != nil {
			continue // already subscribed
		}

		// Create broker handler
		brokerHandler := c.createSubHandler(sub)

		// Build broker subscribe options
		var brokerOpts []broker.SubscribeOption

		if queue := sub.Options().Queue; len(queue) > 0 {
			brokerOpts = append(brokerOpts, broker.Queue(queue))
		}

		if ctx := sub.Options().Context; ctx != nil {
			brokerOpts = append(brokerOpts, broker.SubscribeContext(ctx))
		}

		if !sub.Options().AutoAck {
			brokerOpts = append(brokerOpts, broker.DisableAutoAck())
		}

		c.opts.Logger.Logf(logger.InfoLevel, "Subscribing to topic: %s", sub.Topic())

		// Subscribe to broker
		brokerSub, err := c.opts.Broker.Subscribe(sub.Topic(), brokerHandler, brokerOpts...)
		if err != nil {
			c.opts.Logger.Logf(logger.ErrorLevel, "Failed to subscribe to topic %s: %v", sub.Topic(), err)
			return err
		}

		// Save broker subscriber
		c.subscribers[sub] = brokerSub
	}

	c.started = true
	return nil
}

// Stop unsubscribes all and disconnects
func (c *defaultConsumer) Stop() error {
	c.Lock()

	if !c.started {
		c.Unlock()
		return nil
	}

	// Unsubscribe all
	wg := sync.WaitGroup{}
	for sub, brokerSub := range c.subscribers {
		if brokerSub != nil {
			wg.Add(1)
			go func(s broker.Subscriber) {
				defer wg.Done()
				c.opts.Logger.Logf(logger.InfoLevel, "Unsubscribing from topic: %s", s.Topic())
				s.Unsubscribe()
			}(brokerSub)
		}
		c.subscribers[sub] = nil
	}
	wg.Wait()
	c.Unlock()
	c.started = false
	return nil
}

// Options returns the consumer options
func (c *defaultConsumer) Options() Options {
	c.RLock()
	opts := c.opts
	c.RUnlock()

	return opts
}

// String returns the consumer name
func (c *defaultConsumer) String() string {
	return c.opts.Name
}

// createSubHandler wraps user handler as broker.Handler
func (c *defaultConsumer) createSubHandler(sub *subscriber) broker.Handler {
	return func(e broker.Event) (err error) {
		defer func() {
			if r := recover(); r != nil {
				c.opts.Logger.Logf(logger.ErrorLevel, "panic recovered: %v", r)
				c.opts.Logger.Logf(logger.ErrorLevel, string(debug.Stack()))
				err = fmt.Errorf("%s: panic recovered: %v", c.opts.Name, r)
			}
		}()

		msg := e.Message()
		// if we don't have headers, create empty map
		if msg.Header == nil {
			msg.Header = make(map[string]string)
		}
		// Create consumer message
		consumerMsg := Message{
			Topic:  e.Topic(),
			Header: msg.Header,
			Body:   msg.Body,
		}

		// Create context with headers
		ctx := metadata.NewContext(context.Background(), msg.Header)

		handler := sub.handler
		// Call user handler
		return handler(ctx, consumerMsg)
	}
}
