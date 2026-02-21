package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
)

// SimpleMessage defines the data structure for exchanging messages.
type SimpleMessage struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	// 1. Initialize the milli service and connect to a REAL broker (Kafka)
	srv := milli.NewService(
		milli.Name("example.helloworld"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)), // Mounts Kafka explicitly over default MemoryBroker
	)

	// 2. Define a strongly-typed message handler
	handler := func(ctx context.Context, msg *SimpleMessage) error {
		fmt.Printf("[Consumer] Received Message: ID=%d, Content=%s, Time=%s\n",
			msg.ID, msg.Content, msg.Timestamp.Format(time.RFC3339))
		return nil // AutoAck is true by default, returning nil confirms the message safely
	}

	// 3. Register the subscriber to a topic "hello.topic"
	// milli uses Go Generics [T any] to inject your native type automatically
	if err := milli.RegisterSubscriber("hello.topic", srv, handler); err != nil {
		fmt.Printf("Failed to register subscriber: %v\n", err)
		return
	}

	// 4. Start a background goroutine to publish messages continuously
	go func() {
		// Wait a second for the subscriber to initialize
		time.Sleep(1 * time.Second)

		// Create an Event bound to the topic for convenience
		publisher := milli.NewEvent("hello.topic", srv.Publisher())

		for i := 1; ; i++ {
			msg := &SimpleMessage{
				ID:        i,
				Content:   fmt.Sprintf("Hello Milli %d", i),
				Timestamp: time.Now(),
			}

			fmt.Printf("[Publisher] Sending: %v\n", msg.Content)
			if err := publisher.Publish(context.Background(), msg); err != nil {
				fmt.Printf("Publish error: %v\n", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// 5. Run the service (blocks and listens for SIGTERM to graceful shutdown)
	if err := srv.Run(); err != nil {
		fmt.Printf("Service stopped with error: %v\n", err)
	}
}
