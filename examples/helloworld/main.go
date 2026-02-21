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
)

// SimpleMessage defines the data structure for exchanging messages.
type SimpleMessage struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	// 1. Initialize Zap Logger as the global logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	// 2. Initialize the milli service and connect to a REAL broker (Kafka)
	srv := milli.NewService(
		milli.Name("example.helloworld"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)),
	)

	// 3. Define a strongly-typed message handler
	handler := func(ctx context.Context, msg *SimpleMessage) error {
		logger.Infof("[Consumer] Received Message: ID=%d, Content=%s, Time=%s",
			msg.ID, msg.Content, msg.Timestamp.Format(time.RFC3339))
		return nil
	}

	// 4. Register the subscriber to a topic "hello.topic"
	if err := milli.RegisterSubscriber("hello.topic", srv, handler); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// 5. Start a background goroutine to publish messages continuously
	go func() {
		time.Sleep(1 * time.Second)

		publisher := milli.NewEvent("hello.topic", srv.Publisher())

		for i := 1; ; i++ {
			msg := &SimpleMessage{
				ID:        i,
				Content:   "Hello Milli",
				Timestamp: time.Now(),
			}

			logger.Infof("[Publisher] Sending: %s #%d", msg.Content, msg.ID)
			if err := publisher.Publish(context.Background(), msg); err != nil {
				logger.Errorf("Publish error: %v", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// 6. Run the service (blocks and listens for SIGTERM to graceful shutdown)
	if err := srv.Run(); err != nil {
		logger.Fatalf("Service stopped with error: %v", err)
	}
}
