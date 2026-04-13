package consumer

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/logger"
	"github.com/go-milli/go-milli/metadata"
)

// subscriber represents a topic subscription
type subscriber struct {
	topic   string
	handler Handler
	opts    SubscriberOptions
}

func newSubscriber(topic string, handler Handler, opts ...SubscriberOption) Subscriber {
	options := SubscriberOptions{
		AutoAck: true,
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&options)
	}

	return &subscriber{
		topic:   topic,
		handler: handler,
		opts:    options,
	}
}

func (s *subscriber) Topic() string {
	return s.topic
}

func (s *subscriber) Handler() Handler {
	return s.handler
}

func (s *subscriber) Options() SubscriberOptions {
	return s.opts
}

// createSubHandler wraps user handler as broker.Handler
func (c *defaultConsumer) createSubHandler(sub *subscriber) broker.Handler {
	return func(e broker.Event) (err error) {
		c.wg.Add(1)
		defer c.wg.Done()

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
			Ack:    e.Ack,
		}

		// Create context with headers
		ctx := metadata.NewContext(context.Background(), msg.Header)

		handler := sub.handler
		// Call user handler
		return handler(ctx, consumerMsg)
	}
}
