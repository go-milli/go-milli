package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
)

type ProcessTask struct {
	ID int `json:"id"`
}

func main() {
	// 1. Initialize the service
	srv := milli.NewService(
		milli.Name("example.graceful.shutdown"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)), // Mounts Kafka explicitly over default MemoryBroker
	)

	// 2. Define the handler that takes a relatively long time to finish
	handler := func(ctx context.Context, req *ProcessTask) error {
		fmt.Printf("[Worker] Starting task #%d...\n", req.ID)

		// Simulate a long operation (e.g. 5 seconds)
		// If the process receives a SIGTERM right now, the framework's WaitGroup
		// will ensure it doesn't terminate until this Sleep() finishes.
		time.Sleep(5 * time.Second)

		fmt.Printf("[Worker] Task #%d completed successfully.\n", req.ID)
		return nil
	}

	// 3. Register the subscriber
	if err := milli.RegisterSubscriber("slow.topic", srv, handler); err != nil {
		fmt.Printf("Failed to register subscriber: %v\n", err)
		return
	}

	// 4. Publisher
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("slow.topic", srv.Publisher())

		for i := 1; i <= 3; i++ {
			err := pub.Publish(context.Background(), &ProcessTask{ID: i})
			if err != nil {
				fmt.Printf("[Publisher] Error: %v\n", err)
			}
			fmt.Printf("[Publisher] Dispatched task #%d\n", i)
			time.Sleep(1 * time.Second)
		}

		fmt.Println("[Main] Please press Ctrl+C NOW to observe graceful shutdown! (It should wait for active tasks to finish)")

		// Auto-kill after a few seconds if you don't press Ctrl+C manually
		time.Sleep(2 * time.Second)
		fmt.Println("[Main] Sending SIGTERM to self to simulate Ctrl+C...")
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	}()

	// Run blocks until SIGTERM is received, and gracefully drains waitgroups
	_ = srv.Run()
	fmt.Println("[Main] Server completely and safely shut down.")
}
