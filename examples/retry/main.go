package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/logger"
	zaplogger "github.com/go-milli/go-milli/logger/zap"
	"github.com/go-milli/go-milli/wrapper/retry"
)

type OrderEvent struct {
	OrderID string `json:"order_id"`
	Action  string `json:"action"`
}

// Track per-order attempt counts
var (
	attempts = map[string]int{}
	mu       sync.Mutex
)

func main() {
	// 1. Initialize Zap Logger
	zapLog, err := zaplogger.NewLogger(logger.WithLevel(logger.DebugLevel))
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	logger.DefaultLogger = zapLog
	logger.DefaultHelper = logger.NewHelper(zapLog)

	// 2. Create the Kafka broker (shared for both service and DLQ)
	kb := kafka.NewBroker(broker.Addrs("127.0.0.1:9092"))

	// 3. Initialize the service with Retry middleware
	srv := milli.NewService(
		milli.Name("example.retry"),
		milli.Broker(kb),
		milli.WrapSubscriber(
			// Retry up to 3 times with exponential backoff.
			// After all retries fail, forward to "order.events.dlq" topic.
			retry.NewSubscriberWrapper(
				retry.MaxRetries(3),
				retry.InitialBackoff(500*time.Millisecond),
				retry.DLQ("order.events.dlq", kb),
			),
		),
	)

	// 4. Handler
	//
	// 场景 1: ORD-RECOVER — 前 2 次失败，第 3 次成功 (模拟临时故障恢复)
	//   预期日志:
	//     [Retry] attempt 1/3 failed, retrying in 500ms...
	//     [Retry] attempt 2/3 failed, retrying in 1s...
	//     [BizLogic] ORD-RECOVER processed successfully on attempt 3!
	//
	// 场景 2: ORD-POISON — 永远失败 (模拟毒丸消息，最终进入 DLQ)
	//   预期日志:
	//     [Retry] attempt 1/3 failed, retrying in 500ms...
	//     [Retry] attempt 2/3 failed, retrying in 1s...
	//     [Retry] attempt 3/3 failed, retrying in 2s...
	//     [Retry] all 3 attempts failed
	//     [Retry] message forwarded to DLQ: order.events.dlq
	//
	handler := func(ctx context.Context, msg *OrderEvent) error {
		mu.Lock()
		attempts[msg.OrderID]++
		count := attempts[msg.OrderID]
		mu.Unlock()

		switch msg.OrderID {
		case "ORD-RECOVER":
			// Simulate: fails twice, succeeds on 3rd attempt
			if count < 3 {
				logger.Errorf("[BizLogic] Order %s FAILED (attempt %d) - DB timeout", msg.OrderID, count)
				return fmt.Errorf("temporary DB timeout on attempt %d", count)
			}
			logger.Infof("[BizLogic] Order %s processed successfully on attempt %d!", msg.OrderID, count)
			return nil

		case "ORD-POISON":
			// Simulate: always fails → will be sent to DLQ
			logger.Errorf("[BizLogic] Order %s FAILED (attempt %d) - invalid data", msg.OrderID, count)
			return fmt.Errorf("invalid order data, cannot process")

		default:
			logger.Infof("[BizLogic] Order %s processed normally", msg.OrderID)
			return nil
		}
	}

	// 5. Register subscriber
	if err := milli.RegisterSubscriber("order.events", srv, handler); err != nil {
		logger.Fatalf("Failed to register subscriber: %v", err)
	}

	// 6. Publish two test messages
	go func() {
		time.Sleep(1 * time.Second)
		pub := milli.NewEvent("order.events", srv.Publisher())

		// 场景 1: 会在第 3 次恢复成功
		logger.Info("========== Sending ORD-RECOVER (will succeed after 2 failures) ==========")
		_ = pub.Publish(context.Background(), &OrderEvent{OrderID: "ORD-RECOVER", Action: "CREATE"})

		// 等第一条处理完再发第二条
		time.Sleep(5 * time.Second)

		// 场景 2: 永远失败，最终进 DLQ
		logger.Info("========== Sending ORD-POISON (will exhaust retries → DLQ) ==========")
		_ = pub.Publish(context.Background(), &OrderEvent{OrderID: "ORD-POISON", Action: "CREATE"})
	}()

	_ = srv.Run()
}
