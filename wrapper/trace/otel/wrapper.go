package otel

import (
	"context"
	"fmt"

	"github.com/aevumio/go-milli/consumer"
	"github.com/aevumio/go-milli/publisher"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NewSubscriberWrapper accepts an opentracing Tracer and returns a Subscriber Wrapper.
// This intercepts the incoming message FROM the broker, extracts the upstream TraceID,
// and mounts it as the parent span for the current business processing.
func NewSubscriberWrapper(opts ...Option) consumer.SubscriberWrapper {
	options := NewOptions(opts...)

	return func(next consumer.Handler) consumer.Handler {
		return func(ctx context.Context, msg consumer.Message) error {
			// e.g. "Sub from hello.topic"
			name := fmt.Sprintf("Sub from %s", msg.Topic)
			spanOpts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindConsumer),
			}

			// Open magic tunnel: extract remote parent from metadata
			// (Since createSubHandler already injected msg.Header into ctx's metadata!)

			// Open magic tunnel: extract remote parent from metadata
			ctx, span := StartSpanFromContext(ctx, options.TraceProvider, name, spanOpts...)
			defer span.End()

			// Call the inner handler (business code, or next wrapper)
			if err := next(ctx, msg); err != nil {
				// Record business error on the span
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
				return err
			}

			return nil
		}
	}
}

// NewPublisherWrapper accepts an opentracing Tracer and returns a Publisher Wrapper.
// This intercepts the outgoing message TO the broker, creates a producer span,
// and injects the current trace context into the outgoing message headers (via metadata).
func NewPublisherWrapper(opts ...Option) publisher.Wrapper {
	options := NewOptions(opts...)

	return func(p publisher.Publisher) publisher.Publisher {
		return &otelPublisherWrapper{
			Publisher: p,
			opts:      options,
		}
	}
}

type otelPublisherWrapper struct {
	publisher.Publisher
	opts *Options
}

func (p *otelPublisherWrapper) Publish(ctx context.Context, msg publisher.Message, opts ...publisher.PublishOption) error {
	// e.g. "Pub to hello.topic"
	name := fmt.Sprintf("Pub to %s", msg.Topic())
	spanOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
	}

	// Open magic tunnel: inject the current local span context into the context's metadata!
	ctx, span := StartSpanFromContext(ctx, p.opts.TraceProvider, name, spanOpts...)
	defer span.End()

	// Call the underlying publisher (which will read the very metadata we just injected)
	if err := p.Publisher.Publish(ctx, msg, opts...); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return err
	}

	return nil
}
