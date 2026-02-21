package consumer

import (
	"fmt"
	"sync"

	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/logger"
)

// defaultConsumer implementation
type defaultConsumer struct {
	opts Options

	// subscribers maps subscriber -> broker.Subscriber
	subscribers map[*subscriber]broker.Subscriber

	sync.RWMutex
	// marks the consumer as started
	started bool

	// wg is used for graceful draining of active handlers
	wg sync.WaitGroup
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

	// Apply subscriber wrappers
	fn := handler
	for i := len(c.opts.SubscriberWrappers); i > 0; i-- {
		fn = c.opts.SubscriberWrappers[i-1](fn)
	}

	// Create subscription options
	sb := newSubscriber(topic, fn, opts...)
	sub, ok := sb.(*subscriber)
	if !ok {
		return fmt.Errorf("invalid subscriber: expected *subscriber")
	}
	// Check if already subscribed with same topic AND same queue (consumer group)
	for existing := range c.subscribers {
		if existing.topic == topic && existing.Options().Queue == sub.Options().Queue {
			return fmt.Errorf("already subscribed to topic %s with queue %s", topic, sub.Options().Queue)
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
		} else {
			// Default consumer group = service name, so multiple instances share the same group
			brokerOpts = append(brokerOpts, broker.Queue(c.opts.Name))
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

	// 死死扛住，等待所有正在处理的 handler 任务优雅泄流完毕后再完全关闭
	c.wg.Wait()

	// 断开 broker 底层连接
	if err := c.opts.Broker.Disconnect(); err != nil {
		c.opts.Logger.Logf(logger.ErrorLevel, "Broker disconnect error: %v", err)
	}

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
