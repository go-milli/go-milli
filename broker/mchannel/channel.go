package mchannel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aevumio/go-milli/broker"
	"github.com/aevumio/go-milli/logger"
	"github.com/google/uuid"
)

// Limit is a context key to set the channel queue length limit.
type channelQueueLimitKey struct{}

// WithQueueLimit sets the internal channel buffer size.
func WithQueueLimit(limit int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, channelQueueLimitKey{}, limit)
	}
}

type channelBroker struct {
	opts *broker.Options

	// map of topic to subscribers
	subscribers map[string][]*channelSubscriber

	addr string
	sync.RWMutex
	connected bool
	limit     int
}

type channelEvent struct {
	err     error
	message interface{}
	opts    *broker.Options
	topic   string
}

type channelSubscriber struct {
	opts    broker.SubscribeOptions
	exit    chan bool
	handler broker.Handler
	id      string
	topic   string
	queue   chan *channelEvent
}

func (m *channelBroker) Options() broker.Options {
	return *m.opts
}

func (m *channelBroker) Address() string {
	return m.addr
}

func (m *channelBroker) Connect() error {
	m.Lock()
	defer m.Unlock()

	if m.connected {
		return nil
	}

	id := uuid.New().String()[:8]
	m.addr = fmt.Sprintf("channel://localhost:%s", id)

	m.connected = true

	return nil
}

func (m *channelBroker) Disconnect() error {
	m.Lock()
	defer m.Unlock()

	if !m.connected {
		return nil
	}

	for _, subs := range m.subscribers {
		for _, sub := range subs {
			_ = sub.Unsubscribe()
		}
	}

	m.connected = false

	return nil
}

func (m *channelBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(m.opts)
	}

	if limit, ok := m.opts.Context.Value(channelQueueLimitKey{}).(int); ok {
		m.limit = limit
	}
	return nil
}

func (m *channelBroker) Publish(topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	m.RLock()
	if !m.connected {
		m.RUnlock()
		return errors.New("not connected")
	}

	subs, ok := m.subscribers[topic]
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

	p := &channelEvent{
		topic:   topic,
		message: v,
		opts:    m.opts,
	}

	// This is the queue handling. If subscribers are in the same 'Queue' group,
	// load balancing should be applied. For simplicity, we just push to all individual subscribers
	// unless they share a queue group.
	// Actually to make it pure channel queue:

	// Create groups by queue name
	groups := make(map[string][]*channelSubscriber)
	for _, sub := range subs {
		group := sub.opts.Queue
		if group == "" {
			group = sub.id // unique group if no queue name
		}
		groups[group] = append(groups[group], sub)
	}

	// For each group, we pick a random/round-robin subscriber or push to a shared channel.
	// In our simple channel broker, each subscriber has its own queue.
	// To implement group load-balancing, a proper implementation would use a shared channel per group.
	// But assuming simple pub/sub or 1-to-1 processing:
	for _, groupSubs := range groups {
		// Just send to the first one for now (or round robin)
		// With channel, we could try to send non-blocking:
		sent := false
		for _, sub := range groupSubs {
			select {
			case sub.queue <- p:
				sent = true
			default:
				// channel is full, moving to next in group if any
			}
			if sent {
				break
			}
		}
		if !sent && len(groupSubs) > 0 {
			// If all full, force push to the first one (will block)
			// Wait, we WANT asynchronous non-blocking from Publish's perspective.
			// If channel is full, we must block or drop or spawn goroutine.
			// Spawning a goroutine for a full channel ensures publisher isn't blocked.
			go func(sub *channelSubscriber) {
				sub.queue <- p
			}(groupSubs[0])
		}
	}

	return nil
}

func (m *channelBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	m.RLock()
	if !m.connected {
		m.RUnlock()
		return nil, errors.New("not connected")
	}
	m.RUnlock()

	var options broker.SubscribeOptions
	for _, o := range opts {
		o(&options)
	}

	sub := &channelSubscriber{
		exit:    make(chan bool, 1),
		id:      uuid.New().String(),
		topic:   topic,
		handler: handler,
		opts:    options,
		queue:   make(chan *channelEvent, m.limit),
	}

	m.Lock()
	m.subscribers[topic] = append(m.subscribers[topic], sub)
	m.Unlock()

	// Start consumer goroutine
	go func() {
		for {
			select {
			case <-sub.exit:
				return
			case p := <-sub.queue:
				if err := sub.handler(p); err != nil {
					m.opts.Logger.Logf(logger.ErrorLevel, "[channelBroker] handler error: %v", err)
				}
			}
		}
	}()

	return sub, nil
}

func (m *channelBroker) String() string {
	return "channel"
}

func (m *channelEvent) Topic() string {
	return m.topic
}

func (m *channelEvent) Message() *broker.Message {
	switch v := m.message.(type) {
	case *broker.Message:
		return v
	case []byte:
		msg := &broker.Message{}
		if err := m.opts.Codec.Unmarshal(v, msg); err != nil {
			m.opts.Logger.Logf(logger.ErrorLevel, "[channelBroker]: failed to unmarshal: %v\n", err)
			return nil
		}
		return msg
	}

	return nil
}

func (m *channelEvent) Ack() error {
	return nil
}

func (m *channelEvent) Error() error {
	return m.err
}

func (m *channelSubscriber) Options() broker.SubscribeOptions {
	return m.opts
}

func (m *channelSubscriber) Topic() string {
	return m.topic
}

func (m *channelSubscriber) Unsubscribe() error {
	select {
	case m.exit <- true:
	default:
	}
	return nil
}

// NewBroker returns a new channel-based broker
func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	limit := 1000 // default buffer size
	if l, ok := options.Context.Value(channelQueueLimitKey{}).(int); ok {
		limit = l
	}

	return &channelBroker{
		opts:        options,
		subscribers: make(map[string][]*channelSubscriber),
		limit:       limit,
	}
}
