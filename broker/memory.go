// Package broker provides an in-memory broker for testing and development.
//
// ⚠️ IMPORTANT: This broker is NOT distributed.
// - Messages are stored in local process memory
// - Subscribers must be in the same process
// - NOT suitable for multi-node deployments
//
// Use Cases:
//
//	✅ Unit tests
//	✅ Local development
//	✅ Single-process applications
//	❌ Production environments
//	❌ Distributed systems
//	❌ Multi-node deployments
//
// For production, use real brokers:
//   - Kafka: High throughput, persistence
//   - RabbitMQ: Reliable messaging, routing
package broker

import (
	"errors"
	"fmt"
	"sync"

	"github.com/aevumio/go-milli/logger"
	"github.com/google/uuid"
)

type memoryBroker struct {
	opts *Options

	Subscribers map[string][]*memorySubscriber

	addr string
	sync.RWMutex
	connected bool
}

type memoryEvent struct {
	err     error
	message interface{}
	opts    *Options
	topic   string
}

type memorySubscriber struct {
	opts    SubscribeOptions
	exit    chan bool
	handler Handler
	id      string
	topic   string
}

func (m *memoryBroker) Options() Options {
	return *m.opts
}

func (m *memoryBroker) Address() string {
	return m.addr
}

func (m *memoryBroker) Connect() error {
	m.Lock()
	defer m.Unlock()

	if m.connected {
		return nil
	}

	id := uuid.New().String()[:8]
	m.addr = fmt.Sprintf("memory://localhost:%s", id)

	m.connected = true

	return nil
}

func (m *memoryBroker) Disconnect() error {
	m.Lock()
	defer m.Unlock()

	if !m.connected {
		return nil
	}

	m.connected = false

	return nil
}

func (m *memoryBroker) Init(opts ...Option) error {
	for _, o := range opts {
		o(m.opts)
	}
	return nil
}

func (m *memoryBroker) Publish(topic string, msg *Message, opts ...PublishOption) error {
	m.RLock()
	if !m.connected {
		m.RUnlock()
		return errors.New("not connected")
	}

	subs, ok := m.Subscribers[topic]
	m.RUnlock()
	if !ok {
		return nil
	}

	var v interface{}
	if m.opts.Codec != nil {
		buf, err := m.opts.Codec.Marshal(msg)
		if err != nil {
			return err
		}
		v = buf
	} else {
		v = msg
	}

	p := &memoryEvent{
		topic:   topic,
		message: v,
		opts:    m.opts,
	}

	for _, sub := range subs {
		if err := sub.handler(p); err != nil {
			p.err = err
			return err
		}
	}

	return nil
}

func (m *memoryBroker) Subscribe(topic string, handler Handler, opts ...SubscribeOption) (Subscriber, error) {
	m.RLock()
	if !m.connected {
		m.RUnlock()
		return nil, errors.New("not connected")
	}
	m.RUnlock()

	var options SubscribeOptions
	for _, o := range opts {
		o(&options)
	}

	sub := &memorySubscriber{
		exit:    make(chan bool, 1),
		id:      uuid.New().String(),
		topic:   topic,
		handler: handler,
		opts:    options,
	}

	m.Lock()
	m.Subscribers[topic] = append(m.Subscribers[topic], sub)
	m.Unlock()

	go func() {
		<-sub.exit
		m.Lock()
		var newSubscribers []*memorySubscriber
		for _, sb := range m.Subscribers[topic] {
			if sb.id == sub.id {
				continue
			}
			newSubscribers = append(newSubscribers, sb)
		}
		m.Subscribers[topic] = newSubscribers
		m.Unlock()
	}()

	return sub, nil
}

func (m *memoryBroker) String() string {
	return "memory"
}

func (m *memoryEvent) Topic() string {
	return m.topic
}

func (m *memoryEvent) Message() *Message {
	switch v := m.message.(type) {
	case *Message:
		return v
	case []byte:
		msg := &Message{}
		if err := m.opts.Codec.Unmarshal(v, msg); err != nil {
			m.opts.Logger.Logf(logger.ErrorLevel, "[memory]: failed to unmarshal: %v\n", err)
			return nil
		}
		return msg
	}

	return nil
}

func (m *memoryEvent) Ack() error {
	return nil
}

func (m *memoryEvent) Error() error {
	return m.err
}

func (m *memorySubscriber) Options() SubscribeOptions {
	return m.opts
}

func (m *memorySubscriber) Topic() string {
	return m.topic
}

func (m *memorySubscriber) Unsubscribe() error {
	m.exit <- true
	return nil
}

func NewMemoryBroker(opts ...Option) Broker {
	options := NewOptions(opts...)

	return &memoryBroker{
		opts:        options,
		Subscribers: make(map[string][]*memorySubscriber),
	}
}
