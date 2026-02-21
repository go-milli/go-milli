// Package prometheus provides Prometheus metrics wrappers for go-milli publisher and subscriber.
package prometheus

import (
	"context"
	"time"

	"github.com/aevumio/go-milli/consumer"
	"github.com/aevumio/go-milli/publisher"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Subscriber metrics
	subscriberMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "milli",
			Subsystem: "subscriber",
			Name:      "messages_total",
			Help:      "Total number of messages consumed, partitioned by topic and status.",
		},
		[]string{"topic", "status"},
	)

	subscriberDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "milli",
			Subsystem: "subscriber",
			Name:      "duration_seconds",
			Help:      "Duration of message processing in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"topic"},
	)

	// Publisher metrics
	publisherMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "milli",
			Subsystem: "publisher",
			Name:      "messages_total",
			Help:      "Total number of messages published, partitioned by topic and status.",
		},
		[]string{"topic", "status"},
	)

	publisherDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "milli",
			Subsystem: "publisher",
			Name:      "duration_seconds",
			Help:      "Duration of message publishing in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"topic"},
	)
)

func init() {
	prometheus.MustRegister(
		subscriberMessagesTotal,
		subscriberDuration,
		publisherMessagesTotal,
		publisherDuration,
	)
}

// NewSubscriberWrapper returns a subscriber middleware that collects Prometheus metrics.
func NewSubscriberWrapper() consumer.SubscriberWrapper {
	return func(next consumer.Handler) consumer.Handler {
		return func(ctx context.Context, msg consumer.Message) error {
			start := time.Now()

			err := next(ctx, msg)

			duration := time.Since(start).Seconds()
			subscriberDuration.WithLabelValues(msg.Topic).Observe(duration)

			status := "success"
			if err != nil {
				status = "error"
			}
			subscriberMessagesTotal.WithLabelValues(msg.Topic, status).Inc()

			return err
		}
	}
}

// NewPublisherWrapper returns a publisher middleware that collects Prometheus metrics.
func NewPublisherWrapper() publisher.Wrapper {
	return func(p publisher.Publisher) publisher.Publisher {
		return &publisherMetrics{publisher: p}
	}
}

type publisherMetrics struct {
	publisher publisher.Publisher
}

func (pm *publisherMetrics) Options() publisher.Options {
	return pm.publisher.Options()
}

func (pm *publisherMetrics) Publish(ctx context.Context, msg publisher.Message, opts ...publisher.PublishOption) error {
	start := time.Now()

	err := pm.publisher.Publish(ctx, msg, opts...)

	duration := time.Since(start).Seconds()
	publisherDuration.WithLabelValues(msg.Topic()).Observe(duration)

	status := "success"
	if err != nil {
		status = "error"
	}
	publisherMessagesTotal.WithLabelValues(msg.Topic(), status).Inc()

	return err
}

func (pm *publisherMetrics) String() string {
	return pm.publisher.String()
}
