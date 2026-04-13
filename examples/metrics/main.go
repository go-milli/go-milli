package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/logger"
	zaplogger "github.com/go-milli/go-milli/logger/zap"
	"github.com/go-milli/go-milli/web"
	promwrapper "github.com/go-milli/go-milli/wrapper/metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type OrderEvent struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

func main() {
	// 1. Initialize Zap Logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	kb := kafka.NewBroker(broker.Addrs("127.0.0.1:9092"))

	// ==========================================
	// Part A: Worker (Subscriber with Prometheus metrics)
	// ==========================================
	worker := milli.NewService(
		milli.Name("example.metrics.worker"),
		milli.Broker(kb),
		milli.WrapSubscriber(
			promwrapper.NewSubscriberWrapper(), // 👈 Subscriber metrics
		),
	)

	handler := func(ctx context.Context, msg *OrderEvent) error {
		logger.Infof("[Consumer] Processed order %s, amount: %.2f", msg.OrderID, msg.Amount)
		return nil
	}

	if err := milli.RegisterSubscriber("order.topic", worker, handler); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// Start worker in background (non-blocking)
	go func() {
		if err := worker.Run(); err != nil {
			logger.Errorf("Worker error: %v", err)
		}
	}()

	// ==========================================
	// Part B: Web Gateway (Publisher with Prometheus metrics)
	// ==========================================
	s := web.NewService(
		web.Name("example.metrics.web"),
		web.Address(":8088"),
		web.Mode("debug"),
		web.Broker(kb),
		web.WrapPublisher(
			promwrapper.NewPublisherWrapper(), // 👈 Publisher metrics
		),
	)

	r := s.Engine()

	// Prometheus endpoint — shows BOTH publisher AND subscriber metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.POST("/api/order", func(c *gin.Context) {
		payload := &OrderEvent{
			OrderID: "ORD-" + time.Now().Format("150405"),
			Amount:  99.9,
		}

		event := milli.NewEvent("order.topic", s.Publisher())
		if err := event.Publish(c.Request.Context(), payload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "published", "order_id": payload.OrderID})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test it:
	//   1. curl -X POST http://localhost:8088/api/order    (send a few)
	//   2. curl http://localhost:8088/metrics | grep milli
	//
	// You should see BOTH:
	//   milli_publisher_messages_total{status="success",topic="order.topic"} 3
	//   milli_subscriber_messages_total{status="success",topic="order.topic"} 3
	//   milli_publisher_duration_seconds_sum{topic="order.topic"} 0.015
	//   milli_subscriber_duration_seconds_sum{topic="order.topic"} 0.002

	if err := s.Run(); err != nil {
		logger.Fatal(err)
	}
}
