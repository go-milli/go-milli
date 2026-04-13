package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/consumer"
	"github.com/go-milli/go-milli/logger"
	zaplogger "github.com/go-milli/go-milli/logger/zap"
)

type UserEvent struct {
	UserID string `json:"user_id"`
	Action string `json:"action"`
}

func main() {
	// 1. Initialize Zap Logger as the global logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	// 2. Initialize the service and inject global subscriber middlewares
	srv := milli.NewService(
		milli.Name("example.middleware"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)),
		milli.WrapSubscriber(
			consumer.LoggingMiddleware,
		),
	)

	// 3. Define the strongly typed handler
	handler := func(ctx context.Context, msg *UserEvent) error {
		logger.Infof("[BizLogic] Processing User Action: %s performed %s", msg.UserID, msg.Action)

		if msg.UserID == "error_user" {
			return fmt.Errorf("simulate an error for error_user")
		}
		return nil
	}

	// 4. Register the subscriber
	if err := milli.RegisterSubscriber("user.events", srv, handler); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// 5. Background publisher
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("user.events", srv.Publisher())

		events := []*UserEvent{
			{UserID: "u123", Action: "LOGIN"},
			{UserID: "error_user", Action: "CLICK"},
			{UserID: "u999", Action: "LOGOUT"},
		}

		for _, e := range events {
			logger.Infof("[Publisher] Sending event: %s -> %s", e.UserID, e.Action)
			_ = pub.Publish(context.Background(), e)
			time.Sleep(1 * time.Second)
		}

		time.Sleep(2 * time.Second)
		logger.Info("Example finished.")
	}()

	_ = srv.Run()
}
