package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/wrapper/trace/otel"

	stdotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type TraceMessage struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

// initTracer creates a new trace provider that prints spans to stdout
func initTracer() *sdktrace.TracerProvider {
	// Set the global propagator to TraceContext (the W3C standard for traceparent headers)
	// Without this, otel.GetTextMapPropagator() returns a No-Op propagator!
	stdotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to initialize stdouttrace exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	return tp
}

func main() {
	// 1. Initialize the OpenTelemetry TracerProvider
	tp := initTracer()

	// Register the global Tracer provider
	stdotel.SetTracerProvider(tp)

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 2. Initialize the service with Tracing Wrappers
	srv := milli.NewService(
		milli.Name("example.trace"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)), // Mounts Kafka explicitly over default MemoryBroker

		// Mount Publisher Wrapper (injects Trace ID to outgoing messages)
		// Because we've set the global tracer, no need to pass tp directly!
		milli.WrapPublisher(otel.NewPublisherWrapper()),

		// Mount Subscriber Wrapper (extracts Trace ID from incoming messages)
		milli.WrapSubscriber(otel.NewSubscriberWrapper()),
	)

	// 3. Define the Handler
	handler := func(ctx context.Context, msg *TraceMessage) error {
		// Because of the subscriber wrapper, this handler's `ctx` now contains the extracted span!
		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		fmt.Printf("\n[Consumer] Received traced message: %s\n", msg.Operation)
		fmt.Printf("[Consumer] >> Active TraceID: %s\n", traceID)
		fmt.Printf("[Consumer] >> Active SpanID:  %s\n", spanID)

		return nil
	}

	// 4. Register Subscriber
	if err := milli.RegisterSubscriber("trace.topic", srv, handler); err != nil {
		fmt.Printf("Failed to register subscriber: %v\n", err)
		return
	}

	// 5. Publisher Goroutine
	go func() {
		// Wait for subscriber to completely start
		time.Sleep(1 * time.Second)

		pub := milli.NewEvent("trace.topic", srv.Publisher())

		// Simulate taking a context from a dummy HTTP request
		ctx, span := tp.Tracer("http-server").Start(context.Background(), "HTTP GET /api/trigger")
		defer span.End()

		msg := &TraceMessage{ID: "TRC-01", Operation: "InitiatePayment"}
		fmt.Printf("\n[Publisher] Sending message inside existing Trace Span...\n")

		// We pass the active trace context into the Publisher.
		// The PublisherWrapper will magically inject this context's trace-id into the broker headers!
		_ = pub.Publish(ctx, msg)

		time.Sleep(3 * time.Second)
		fmt.Printf("\n[Main] Exiting trace example. Check stdout for the beautiful OTel Span outputs showing parenthood (TraceID connection)!\n\n")
		// The service run blocks, so we'll simulate an interrupt if desired
	}()

	_ = srv.Run()
}
