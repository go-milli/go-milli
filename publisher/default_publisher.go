package publisher

import (
	"context"
	"fmt"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/metadata.go"
)

type defaultPublisher struct {
	opts Options
}

// NewDefaultPublisher creates a new default publisher
func NewDefaultPublisher(opts ...Option) Publisher {
	options := NewOptions(opts...)

	var p Publisher = &defaultPublisher{
		opts: options,
	}

	for i := len(options.Wrappers); i > 0; i-- {
		p = options.Wrappers[i-1](p)
	}

	return p
}

func (p *defaultPublisher) Options() Options {
	return p.opts
}

func (p *defaultPublisher) Publish(ctx context.Context, msg Message, opts ...PublishOption) error {
	options := NewPublishOptions(opts...)

	// 如果 PublishOption 里的 Context 为空，则回退使用传入的 Context
	if options.Context == nil {
		options.Context = ctx
	}

	// 1. 从 context 中提取 metadata 作为 headers
	var headers map[string]string
	if md, ok := metadata.FromContext(options.Context); ok {
		headers = make(map[string]string, len(md))
		for k, v := range md {
			headers[k] = v
		}
	} else {
		headers = make(map[string]string)
	}

	// 补充 Content-Type 信息 (非常重要的协议语义)
	contentType := msg.ContentType()
	if contentType == "" {
		contentType = p.opts.Codec.String()
	}
	headers["Content-Type"] = contentType

	// 2. 将 msg.Payload() (通常是 struct 或者是 []byte) 序列化
	var body []byte
	var err error

	switch v := msg.Payload().(type) {
	case []byte:
		body = v
	case string:
		body = []byte(v)
	default:
		body, err = p.opts.Codec.Marshal(msg.Payload())
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}

	// 3. 构建底层 broker 需要的 Message
	brokerMsg := &broker.Message{
		Header: headers,
		Body:   body,
	}

	// 4. 调用底层的 broker 发送消息
	err = p.opts.Broker.Publish(msg.Topic(), brokerMsg)
	if err != nil {
		return fmt.Errorf("failed to publish to broker: %w", err)
	}

	return nil
}

func (p *defaultPublisher) String() string {
	return p.opts.Name
}
