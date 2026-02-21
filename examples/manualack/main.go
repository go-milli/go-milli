package main

import (
	"context"
	"log"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/consumer"
	"github.com/go-milli/go-milli/logger"
	zaplogger "github.com/go-milli/go-milli/logger/zap"
)

type PaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

func main() {
	// 1. Initialize Zap Logger as the global logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	// 2. Initialize the service with ContextAckWrapper
	srv := milli.NewService(
		milli.Name("example.manualack"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)),
		milli.WrapSubscriber(consumer.ContextAckWrapper),
	)

	// 3. Define the handler
	handler := func(ctx context.Context, req *PaymentRequest) error {
		logger.Infof("[BizLogic] Received order %s... Processing in background.", req.OrderID)

		go func(orderReq *PaymentRequest) {
			// Simulate a long-running DB transaction
			time.Sleep(2 * time.Second)

			logger.Infof("[Async Worker] Order %s successfully saved to DB.", orderReq.OrderID)

			// Trigger Manual ACK
			err := milli.Ack(ctx)
			if err != nil {
				logger.Errorf("[Async Worker] Failed to ack order %s: %v", orderReq.OrderID, err)
			} else {
				logger.Infof("[Async Worker] Order %s manually ACKED to Broker!", orderReq.OrderID)
			}
		}(req)

		return nil
	}

	// 4. Register the subscriber with DisableAutoAck
	if err := milli.RegisterSubscriber("payment.topic", srv, handler, consumer.DisableAutoAck()); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// 5. Publish one message for testing
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("payment.topic", srv.Publisher())
		msg := &PaymentRequest{OrderID: "ORD-9981", Amount: 50.2}

		logger.Infof("[Publisher] Publishing order %s", msg.OrderID)
		_ = pub.Publish(context.Background(), msg)

		time.Sleep(5 * time.Second)
	}()

	_ = srv.Run()
}
