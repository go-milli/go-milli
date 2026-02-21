// Package retry provides a subscriber middleware for automatic retry with exponential backoff and optional DLQ.
package retry

import (
	"context"
	"time"

	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/consumer"
	"github.com/aevumio/go-milli/logger"
)

// Options configures the retry middleware.
type Options struct {
	// MaxRetries is the maximum number of retry attempts. Default: 3
	MaxRetries int

	// InitialBackoff is the delay before the first retry. Default: 500ms
	InitialBackoff time.Duration

	// MaxBackoff caps the maximum backoff duration. Default: 10s
	MaxBackoff time.Duration

	// Multiplier is the factor by which backoff increases each retry. Default: 2.0
	Multiplier float64

	// DLQTopic is the dead letter queue topic. If set, failed messages are forwarded here after all retries are exhausted.
	DLQTopic string

	// DLQBroker is the broker used to publish to the DLQ topic. Required if DLQTopic is set.
	DLQBroker broker.Broker
}

type Option func(*Options)

func newOptions(opts ...Option) Options {
	o := Options{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// MaxRetries sets the maximum number of retry attempts.
func MaxRetries(n int) Option {
	return func(o *Options) { o.MaxRetries = n }
}

// InitialBackoff sets the delay before the first retry.
func InitialBackoff(d time.Duration) Option {
	return func(o *Options) { o.InitialBackoff = d }
}

// MaxBackoff caps the maximum backoff duration.
func MaxBackoff(d time.Duration) Option {
	return func(o *Options) { o.MaxBackoff = d }
}

// Multiplier sets the backoff multiplier.
func Multiplier(m float64) Option {
	return func(o *Options) { o.Multiplier = m }
}

// DLQ configures the dead letter queue topic and broker.
func DLQ(topic string, b broker.Broker) Option {
	return func(o *Options) {
		o.DLQTopic = topic
		o.DLQBroker = b
	}
}

// NewSubscriberWrapper creates a retry middleware with the given options.
func NewSubscriberWrapper(opts ...Option) consumer.SubscriberWrapper {
	options := newOptions(opts...)

	return func(next consumer.Handler) consumer.Handler {
		return func(ctx context.Context, msg consumer.Message) error {
			var lastErr error

			for attempt := 0; attempt <= options.MaxRetries; attempt++ {
				lastErr = next(ctx, msg)
				if lastErr == nil {
					return nil // Success!
				}

				// If this was the last attempt, don't sleep
				if attempt == options.MaxRetries {
					break
				}

				// Calculate backoff
				backoff := time.Duration(float64(options.InitialBackoff) * pow(options.Multiplier, float64(attempt)))
				if backoff > options.MaxBackoff {
					backoff = options.MaxBackoff
				}

				logger.Infof("[Retry] attempt %d/%d failed for topic %s, retrying in %v: %v",
					attempt+1, options.MaxRetries, msg.Topic, backoff, lastErr)

				time.Sleep(backoff)
			}

			// All retries exhausted
			logger.Errorf("[Retry] all %d attempts failed for topic %s: %v", options.MaxRetries, msg.Topic, lastErr)

			// Send to DLQ if configured
			if options.DLQTopic != "" && options.DLQBroker != nil {
				dlqMsg := &broker.Message{
					Header: msg.Header,
					Body:   msg.Body,
				}
				dlqMsg.Header["X-Original-Topic"] = msg.Topic
				dlqMsg.Header["X-Last-Error"] = lastErr.Error()

				if err := options.DLQBroker.Publish(options.DLQTopic, dlqMsg); err != nil {
					logger.Errorf("[Retry] failed to publish to DLQ %s: %v", options.DLQTopic, err)
				} else {
					logger.Infof("[Retry] message forwarded to DLQ: %s", options.DLQTopic)
				}

				// Return nil so the message gets acked (it's in DLQ now, not lost)
				return nil
			}

			return lastErr
		}
	}
}

// pow calculates base^exp without importing math.
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
