package main

import (
	"fmt"
	"time"

	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/broker/kafka"
)

func main() {
	b := kafka.NewBroker(
		broker.Addrs("127.0.0.1:9092"),
	)

	err := b.Connect()
	if err != nil {
		panic(err)
	}
	defer b.Disconnect()

	// 订阅消息
	s, err := b.Subscribe("test.milli", func(e broker.Event) error {
		fmt.Println("收到消息 Header:", e.Message().Header)
		fmt.Println("收到消息 Body:", string(e.Message().Body))
		return nil
	})
	if err != nil {
		panic(err)
	}
	defer s.Unsubscribe()

	// 发布消息（可以在 goroutine 里，避免阻塞订阅）
	go func() {
		// 等待 Consumer Group 准备好
		time.Sleep(10 * time.Second)
		fmt.Println("开始发布消息...")

		for i := 0; i < 5; i++ {
			msg := &broker.Message{
				Header: map[string]string{
					"Trace-Id": fmt.Sprintf("trace-%d", i),
				},
				Body: []byte(fmt.Sprintf("Hello Kafka %d", i)),
			}

			if err := b.Publish("test.milli", msg); err != nil {
				fmt.Println("发布失败:", err)
			} else {
				fmt.Println("已发布消息:", string(msg.Body))
			}

			time.Sleep(time.Second)
		}
	}()

	// 阻塞 main，让订阅器持续运行
	select {}
}
