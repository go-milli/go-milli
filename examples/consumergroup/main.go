// Consumer Group 示例
//
// 本例演示 go-milli 的 Consumer Group 行为：
//
// 1. 默认行为（推荐）：
//    Consumer Group 名 = Service Name（例如 "order.worker"）
//    同名 Service 的多个实例自动共享消费，实现负载均衡。
//    重启不换 Group 名，Kafka 从上次 offset 继续消费，不重复。
//
// 2. 自定义 Group：
//    通过 consumer.SubscriberQueue("my-custom-group") 手动指定。
//    适合一个 Service 需要以不同 Group 消费同一个 Topic 的场景。
//
// 测试方式：
//   终端 A:  go run main.go
//   终端 B:  go run main.go   (同一个 binary 启动第二个实例)
//
//   两个实例会共享 "order.worker" 消费组，
//   发送的消息只会被其中一个消费到（负载均衡），而不是两个都消费（广播）。

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

	// 2. Create service
	//    Consumer Group 默认 = Service Name = "order.worker"
	//    多个实例启动后自动共享消费（负载均衡）
	srv := milli.NewService(
		milli.Name("order.worker"),
		milli.Broker(kafka.NewBroker(
			broker.Addrs("127.0.0.1:9092"),
		)),
	)

	// 3. 场景 A: 默认 Group（推荐，Group = "order.worker"）
	defaultHandler := func(ctx context.Context, msg *OrderEvent) error {
		logger.Infof("[Default Group] Received order %s, amount: %.2f", msg.OrderID, msg.Amount)
		return nil
	}
	if err := milli.RegisterSubscriber("order.topic", srv, defaultHandler); err != nil {
		logger.Fatalf("Failed to register default subscriber: %v", err)
	}

	// 4. 场景 B: 自定义 Group（适用于同一个 Topic 需要多种消费方式的场景）
	//    比如：一路消费用于实时处理，另一路消费用于归档
	archiveHandler := func(ctx context.Context, msg *OrderEvent) error {
		logger.Infof("[Archive Group] Archiving order %s", msg.OrderID)
		return nil
	}
	if err := milli.RegisterSubscriber("order.topic", srv, archiveHandler,
		consumer.SubscriberQueue("order.archive"), // 自定义 Group 名
	); err != nil {
		logger.Fatalf("Failed to register archive subscriber: %v", err)
	}

	// 5. Publish test messages
	go func() {
		time.Sleep(2 * time.Second)
		pub := milli.NewEvent("order.topic", srv.Publisher())

		for i := 1; i <= 5; i++ {
			msg := &OrderEvent{
				OrderID: "ORD-" + time.Now().Format("150405"),
				Amount:  float64(i) * 10.5,
			}
			logger.Infof("[Publisher] Sending order %s", msg.OrderID)
			_ = pub.Publish(context.Background(), msg)
			time.Sleep(1 * time.Second)
		}
	}()

	// 每条消息会被消费两次：
	//   一次被 "order.worker" group (场景 A)
	//   一次被 "order.archive" group (场景 B)
	// 因为两个 group 是独立的消费视角！

	_ = srv.Run()
}
