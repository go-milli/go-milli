package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/aevumio/go-milli"
	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/broker/kafka"
	"github.com/aevumio/go-milli/logger"
	zaplogger "github.com/aevumio/go-milli/logger/zap"
	"github.com/aevumio/go-milli/web"
	"github.com/aevumio/go-milli/wrapper/trace/otel"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	stdotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func initTracer() *sdktrace.TracerProvider {
	stdotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to initialize stdouttrace exporter: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	stdotel.SetTracerProvider(tp)
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

	// 2. Initialize OTel
	tp := initTracer()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// 3. Create web service (Gin + optional Publisher, all-in-one)
	s := web.NewService(
		web.Name("example.web"),
		web.Address(":8088"),
		web.Mode("debug"),
		web.Broker(kafka.NewBroker(broker.Addrs("127.0.0.1:9092"))),
		web.WrapPublisher(otel.NewPublisherWrapper()),
	)

	// 4. Get the raw gin.Engine and mount middlewares / routes
	r := s.Engine()
	r.Use(otelgin.Middleware("example.web"))

	r.POST("/api/order", func(c *gin.Context) {
		ctx := c.Request.Context()
		span := trace.SpanFromContext(ctx)

		// Use our logger with trace context
		h := logger.DefaultHelper.WithFields(map[string]interface{}{
			"trace_id": span.SpanContext().TraceID().String(),
			"path":     c.Request.URL.Path,
		})
		h.Info("Received order request")

		payload := map[string]interface{}{
			"order_id": "ODR-" + time.Now().Format("150405"),
			"amount":   100.5,
		}

		// Publish to Kafka with full trace context propagation
		event := milli.NewEvent("order.topic", s.Publisher())
		if err := event.Publish(ctx, payload); err != nil {
			h.WithError(err).Error("Failed to publish order")
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		h.Info("Order dispatched successfully")
		c.JSON(200, gin.H{
			"status":   "Order dispatched!",
			"trace_id": span.SpanContext().TraceID().String(),
		})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 5. Run! (Broker connect, HTTP listen, graceful shutdown — all automatic)
	if err := s.Run(); err != nil {
		logger.Fatal(err)
	}
}
