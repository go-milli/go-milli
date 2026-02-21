package milli

import (
	"context"

	"github.com/go-milli/go-milli/consumer"
	"github.com/go-milli/go-milli/publisher"
)

// Service 是顶级容器门面接口
type Service interface {
	Name() string
	// Consumer 返回底层的消费调度器（通常用户不需要直接调用它）
	Consumer() consumer.Consumer

	// Publisher 返回底层的发布调度引擎
	Publisher() publisher.Publisher

	Run() error
}

// Event 是提供给用户的便捷封装，绑定了特定的 Topic
type Event interface {
	Publish(ctx context.Context, msg any, opts ...publisher.PublishOption) error
}

// topicEvent 是 Event 接口的默认实现
type topicEvent struct {
	topic string
	pub   publisher.Publisher
}

// Publish 实现了 Event 接口，内部自动把 topic 传给底层的 Publisher
func (e *topicEvent) Publish(ctx context.Context, msg any, opts ...publisher.PublishOption) error {
	// 创建一个业务层的 Message，contentType 留空让底层 Publisher 从 Codec 获取默认值
	pubMsg := publisher.NewMessage(e.topic, msg, "")
	return e.pub.Publish(ctx, pubMsg, opts...)
}

// NewEvent 是一个语法糖，帮用户快速组装一个绑定了 Topic 的发布对象
// 对应原版 go-micro 里的 micro.NewEvent
func NewEvent(topic string, p publisher.Publisher) Event {
	return &topicEvent{
		topic: topic,
		pub:   p,
	}
}

// RegisterSubscriber 是泛型语法糖！
// 它把业务方强类型（T）的 Handler 包装成 consumer 需要的原始 Handler。
func RegisterSubscriber[T any](topic string, srv Service, handler func(context.Context, *T) error, opts ...consumer.SubscriberOption) error {
	c := srv.Consumer()
	codec := c.Options().Codec

	rawHandler := func(ctx context.Context, msg consumer.Message) error {
		var req T
		if codec != nil {
			if err := codec.Unmarshal(msg.Body, &req); err != nil {
				// 有可以的话可以加一行 log
				return err
			}
		}
		return handler(ctx, &req)
	}

	return c.Subscribe(topic, rawHandler, opts...)
}

// Ack 是一键手动确认的语法糖，用于在禁用了自动 Ack 时（DisableAutoAck），手动提取 Context 里由 ContextAckWrapper 埋入的 Ack 闭包执行确认。
func Ack(ctx context.Context) error {
	return consumer.Ack(ctx)
}
