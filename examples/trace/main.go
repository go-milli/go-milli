package main

import (
	"context"
	"log"
	"time"

	"github.com/aevumio/go-milli"
	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/broker/kafka"
	"github.com/aevumio/go-milli/logger"
	zaplogger "github.com/aevumio/go-milli/logger/zap"
	"github.com/aevumio/go-milli/wrapper/trace/otel"

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
	// 1. Initialize Zap Logger as the global logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	// 2. Initialize the OpenTelemetry TracerProvider
	tp := initTracer()
	stdotel.SetTracerProvider(tp)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 3. Initialize the service with Tracing Wrappers
	srv := milli.NewService(
		milli.Name("example.trace"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)),
		milli.WrapPublisher(otel.NewPublisherWrapper()),
		milli.WrapSubscriber(otel.NewSubscriberWrapper()),
	)

	// 4. Define the Handler
	handler := func(ctx context.Context, msg *TraceMessage) error {
		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		logger.DefaultHelper.WithFields(map[string]interface{}{
			"trace_id":  traceID,
			"span_id":   spanID,
			"operation": msg.Operation,
		}).Info("[Consumer] Received traced message")

		return nil
	}

	// 5. Register Subscriber
	if err := milli.RegisterSubscriber("trace.topic", srv, handler); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// 6. Publisher Goroutine
	go func() {
		time.Sleep(1 * time.Second)

		pub := milli.NewEvent("trace.topic", srv.Publisher())

		// Simulate taking a context from a dummy HTTP request
		ctx, span := tp.Tracer("http-server").Start(context.Background(), "HTTP GET /api/trigger")
		defer span.End()

		msg := &TraceMessage{ID: "TRC-01", Operation: "InitiatePayment"}
		logger.Info("[Publisher] Sending message inside existing Trace Span...")

		_ = pub.Publish(ctx, msg)

		time.Sleep(3 * time.Second)
		logger.Info("[Main] Exiting trace example. Check stdout for OTel Span outputs!")
	}()

	_ = srv.Run()
}
