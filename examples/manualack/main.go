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

type PaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

func main() {
	// 1. Initialize the service with ContextAckWrapper
	// This wrapper is essential to inject the low-level `broker.Event.Ack` closure
	// into the context of the high-level strongly-typed `func(ctx, req)`.
	srv := milli.NewService(
		milli.Name("example.manualack"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)), // Mounts Kafka explicitly over default MemoryBroker
		milli.WrapSubscriber(consumer.ContextAckWrapper),
	)

	// 2. Define the handler
	handler := func(ctx context.Context, req *PaymentRequest) error {
		fmt.Printf("[BizLogic] Received order %s... Processing in background.\n", req.OrderID)

		// Imagine we process this asynchronously and thus must release the main MQ handler routine quickly
		go func(orderReq *PaymentRequest) {
			// Simulate a simulated long-running DB transaction
			time.Sleep(2 * time.Second)

			// Done processing! It's safe to manually acknowledge now.
			fmt.Printf("[Async Worker] Order %s successfully saved to DB.\n", orderReq.OrderID)

			// Trigger Manual ACK by extracting the hidden closure using our milli.Ack helper
			err := milli.Ack(ctx)
			if err != nil {
				fmt.Printf("[Async Worker] Failed to ack order %s: %v\n", orderReq.OrderID, err)
			} else {
				fmt.Printf("[Async Worker] Order %s manually ACKED to Broker!\n", orderReq.OrderID)
			}
		}(req)

		// Returns immediately without causing default auto-ack (if AutoAck was true, this would be dangerous here!)
		// Because we pass DisableAutoAck during subscription, returning nil here does nothing on the broker.
		return nil
	}

	// 3. Register the subscriber and pass the VERY IMPORTANT `DisableAutoAck` option!
	if err := milli.RegisterSubscriber("payment.topic", srv, handler, consumer.DisableAutoAck()); err != nil {
		fmt.Printf("Failed to register subscriber: %v\n", err)
		return
	}

	// Publish one message for testing
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("payment.topic", srv.Publisher())
		msg := &PaymentRequest{OrderID: "ORD-9981", Amount: 50.2}

		fmt.Printf("[Publisher] Publishing order %s\n", msg.OrderID)
		_ = pub.Publish(context.Background(), msg)

		// Stop the service after 5 seconds
		time.Sleep(5 * time.Second)
		// Usually we stop with SIGTERM, but we can't easily trigger it. Srv.Stop() doesn't exist natively on the facade.
	}()

	_ = srv.Run()
}
