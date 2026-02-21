package otel

import (
	"context"
	"strings"

	"github.com/go-milli/go-milli/metadata"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "github.com/go-milli/go-milli/wrapper/trace/otel"
)

// StartSpanFromContext returns a new span with the given operation name and options. If a span
// is found in the context (usually via Context propagation metadata), it will be used as the parent.
func StartSpanFromContext(
	ctx context.Context,
	tp trace.TracerProvider,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {

	// 1. Extract metadata from the incoming context.
	// For consumer: this came magically from Broker Headers.
	// For publisher: this might have come from somewhere upstream (e.g. HTTP layer).
	md, ok := metadata.FromContext(ctx)
	if !ok {
		md = make(metadata.Metadata)
	}

	// 2. Prepare OTel TextMapPropagator and Carrier
	propagator := otel.GetTextMapPropagator()
	carrier := make(propagation.MapCarrier)

	// Transfer milli metadata to OTel MapCarrier for extraction
	for k, v := range md {
		for _, f := range propagator.Fields() {
			if strings.EqualFold(k, f) {
				carrier[f] = v
			}
		}
	}

	// 3. Let OTel Extract the parent SpanContext from the Carrier
	ctx = propagator.Extract(ctx, carrier)
	spanCtx := trace.SpanContextFromContext(ctx)
	ctx = baggage.ContextWithBaggage(ctx, baggage.FromContext(ctx))

	// 4. Start the new Span
	var tracer trace.Tracer
	if tp != nil {
		tracer = tp.Tracer(instrumentationName)
	} else {
		tracer = otel.Tracer(instrumentationName)
	}

	// We create the span, naturally linking to spanCtx (which will be remote if extracted)
	ctx, span := tracer.Start(trace.ContextWithRemoteSpanContext(ctx, spanCtx), name, opts...)

	// 5. Inject the newly created Span information BACK into the context metadata!
	// This is the crucial step required for the next hop (e.g. Publisher -> network)
	carrier = make(propagation.MapCarrier)
	propagator.Inject(ctx, carrier)

	for k, v := range carrier {
		// e.g. "traceparent" -> "00-xxx-xxx-01"
		md[k] = v
	}

	// Return the new context wrapping the updated metadata and the new span
	ctx = metadata.NewContext(ctx, md)

	return ctx, span
}
