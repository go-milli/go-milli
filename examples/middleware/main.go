package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/consumer"
)

type UserEvent struct {
	UserID string `json:"user_id"`
	Action string `json:"action"`
}

func main() {
	// 1. Initialize the service and inject global subscriber middlewares
	srv := milli.NewService(
		milli.Name("example.middleware"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)), // Mounts Kafka explicitly over default MemoryBroker
		// The outer-most wrapper is executed first in the onion ring,
		// but since we range backwards, Tracing wraps Logging, which wraps the actual Handler.
		milli.WrapSubscriber(
			consumer.TracingMiddleware, // First layer: Extract Tracing Header & Print Start/End
			consumer.LoggingMiddleware, // Second layer: Print Topic logs
		),
	)

	// 2. Define the strongly typed handler
	handler := func(ctx context.Context, msg *UserEvent) error {
		// In a real application, Context would hold the trace IDs injected by TracingMiddleware
		fmt.Printf("  -> [BizLogic] Processing User Action: %s performed %s\n", msg.UserID, msg.Action)

		if msg.UserID == "error_user" {
			return fmt.Errorf("simulate an error for error_user")
		}
		return nil
	}

	// 3. Register the subscriber
	if err := milli.RegisterSubscriber("user.events", srv, handler); err != nil {
		fmt.Printf("Failed to register subscriber: %v\n", err)
		return
	}

	// 4. Background publisher
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("user.events", srv.Publisher())

		events := []*UserEvent{
			{UserID: "u123", Action: "LOGIN"},
			{UserID: "error_user", Action: "CLICK"}, // Will trigger error in middleware
			{UserID: "u999", Action: "LOGOUT"},
		}

		for _, e := range events {
			_ = pub.Publish(context.Background(), e)
			time.Sleep(1 * time.Second)
		}

		// Wait a bit and shutdown
		time.Sleep(2 * time.Second)
		fmt.Println("Example finished.")
	}()

	// Run
	_ = srv.Run()
}
