// Package redis provides a Redis Streams based broker implementation.
//
// This broker uses Redis Streams (XADD/XREADGROUP/XACK) to provide
// a lightweight, persistent message queue that supports:
//   - Consumer groups for load balancing across multiple workers
//   - Message acknowledgement (at-least-once delivery)
//   - Message persistence (survives process restarts)
//   - Batch consumption via COUNT parameter
//
// Use Cases:
//
//	✅ Single-node and multi-node deployments
//	✅ Production environments (with Redis persistence)
//	✅ Lightweight alternative to Kafka
//	✅ Projects that already have Redis
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/logger"
	"github.com/redis/go-redis/v9"
)

// ---- Option keys ----

type redisClientKey struct{}
type consumerGroupKey struct{}
type consumerNameKey struct{}
type blockTimeoutKey struct{}
type batchCountKey struct{}

// WithRedisClient sets a pre-configured *redis.Client.
func WithRedisClient(client *redis.Client) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, redisClientKey{}, client)
	}
}

// WithConsumerGroup sets the consumer group name (default: "default").
func WithConsumerGroup(group string) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, consumerGroupKey{}, group)
	}
}

// WithConsumerName sets the consumer name within the group (default: auto-generated).
func WithConsumerName(name string) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, consumerNameKey{}, name)
	}
}

// WithBlockTimeout sets how long XREADGROUP blocks waiting for new messages (default: 3s).
func WithBlockTimeout(d time.Duration) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, blockTimeoutKey{}, d)
	}
}

// WithBatchCount sets how many messages to read per XREADGROUP call (default: 100).
func WithBatchCount(count int64) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, batchCountKey{}, count)
	}
}

// ---- Broker implementation ----

type redisBroker struct {
	opts *broker.Options

	client        *redis.Client
	consumerGroup string
	consumerName  string
	blockTimeout  time.Duration
	batchCount    int64

	subscribers []*redisSubscriber

	sync.RWMutex
	connected bool
}

type redisEvent struct {
	topic   string
	message *broker.Message
	client  *redis.Client
	stream  string
	msgID   string
	group   string
}

type redisSubscriber struct {
	opts    broker.SubscribeOptions
	topic   string
	exit    chan struct{}
	handler broker.Handler
	done    chan struct{}
}

func (b *redisBroker) Options() broker.Options {
	return *b.opts
}

func (b *redisBroker) Address() string {
	return b.client.Options().Addr
}

func (b *redisBroker) Connect() error {
	b.Lock()
	defer b.Unlock()

	if b.connected {
		return nil
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	b.connected = true
	return nil
}

func (b *redisBroker) Disconnect() error {
	b.Lock()
	defer b.Unlock()

	if !b.connected {
		return nil
	}

	// Stop all subscribers
	for _, sub := range b.subscribers {
		close(sub.exit)
		<-sub.done // wait for goroutine to finish
	}
	b.subscribers = nil

	b.connected = false
	return nil
}

func (b *redisBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(b.opts)
	}
	b.applyOptions()
	return nil
}

func (b *redisBroker) Publish(topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	b.RLock()
	if !b.connected {
		b.RUnlock()
		return errors.New("not connected")
	}
	b.RUnlock()

	// Build fields for XADD
	fields := make(map[string]interface{})
	fields["body"] = string(msg.Body)

	// Encode headers as "hdr.<key>" fields
	for k, v := range msg.Header {
		fields["hdr."+k] = v
	}

	ctx := context.Background()
	_, err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: topic,
		Values: fields,
	}).Result()
	if err != nil {
		return fmt.Errorf("XADD to %s failed: %w", topic, err)
	}

	return nil
}

func (b *redisBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	b.RLock()
	if !b.connected {
		b.RUnlock()
		return nil, errors.New("not connected")
	}
	b.RUnlock()

	var options broker.SubscribeOptions
	for _, o := range opts {
		o(&options)
	}

	// Determine consumer group
	group := b.consumerGroup
	if options.Queue != "" {
		group = options.Queue
	}

	// Ensure consumer group exists (XGROUP CREATE, ignore if exists)
	ctx := context.Background()
	err := b.client.XGroupCreateMkStream(ctx, topic, group, "0").Err()
	if err != nil && !isGroupExistsError(err) {
		return nil, fmt.Errorf("XGROUP CREATE failed: %w", err)
	}

	sub := &redisSubscriber{
		opts:    options,
		topic:   topic,
		exit:    make(chan struct{}),
		handler: handler,
		done:    make(chan struct{}),
	}

	b.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.Unlock()

	// Start consumer goroutine
	go b.consumeLoop(sub, topic, group)

	return sub, nil
}

func (b *redisBroker) consumeLoop(sub *redisSubscriber, stream, group string) {
	defer close(sub.done)

	for {
		select {
		case <-sub.exit:
			return
		default:
		}

		ctx := context.Background()
		results, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: b.consumerName,
			Streams:  []string{stream, ">"},
			Count:    b.batchCount,
			Block:    b.blockTimeout,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				// Timeout, no new messages — loop again
				continue
			}
			b.opts.Logger.Logf(logger.ErrorLevel, "[redisBroker] XREADGROUP error: %v", err)
			time.Sleep(1 * time.Second) // backoff on error
			continue
		}

		for _, xstream := range results {
			for _, xmsg := range xstream.Messages {
				// Reconstruct broker.Message
				msg := &broker.Message{
					Header: make(map[string]string),
				}

				for k, v := range xmsg.Values {
					val, _ := v.(string)
					if k == "body" {
						msg.Body = []byte(val)
					} else if strings.HasPrefix(k, "hdr.") {
						msg.Header[k[4:]] = val
					}
				}

				evt := &redisEvent{
					topic:   stream,
					message: msg,
					client:  b.client,
					stream:  stream,
					msgID:   xmsg.ID,
					group:   group,
				}

				if err := sub.handler(evt); err != nil {
					b.opts.Logger.Logf(logger.ErrorLevel, "[redisBroker] handler error for msg %s: %v", xmsg.ID, err)
					// Don't ACK — message stays pending for retry
					continue
				}

				// ACK on success
				if sub.opts.AutoAck {
					_ = evt.Ack()
				}
			}
		}
	}
}

func (b *redisBroker) String() string {
	return "redis"
}

func (b *redisBroker) applyOptions() {
	if client, ok := b.opts.Context.Value(redisClientKey{}).(*redis.Client); ok {
		b.client = client
	}
	if group, ok := b.opts.Context.Value(consumerGroupKey{}).(string); ok {
		b.consumerGroup = group
	}
	if name, ok := b.opts.Context.Value(consumerNameKey{}).(string); ok {
		b.consumerName = name
	}
	if timeout, ok := b.opts.Context.Value(blockTimeoutKey{}).(time.Duration); ok {
		b.blockTimeout = timeout
	}
	if count, ok := b.opts.Context.Value(batchCountKey{}).(int64); ok {
		b.batchCount = count
	}
}

// ---- Event implementation ----

func (e *redisEvent) Topic() string {
	return e.topic
}

func (e *redisEvent) Message() *broker.Message {
	return e.message
}

func (e *redisEvent) Ack() error {
	return e.client.XAck(context.Background(), e.stream, e.group, e.msgID).Err()
}

func (e *redisEvent) Error() error {
	return nil
}

// ---- Subscriber implementation ----

func (s *redisSubscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *redisSubscriber) Topic() string {
	return s.topic
}

func (s *redisSubscriber) Unsubscribe() error {
	select {
	case <-s.exit:
		// already closed
	default:
		close(s.exit)
	}
	<-s.done // wait for goroutine to finish
	return nil
}

// ---- Constructor ----

// NewBroker returns a new Redis Streams based broker.
// If no WithRedisClient option is provided, a default client is created using broker.Addrs.
func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	b := &redisBroker{
		opts:          options,
		consumerGroup: "default",
		consumerName:  fmt.Sprintf("consumer-%d", time.Now().UnixNano()),
		blockTimeout:  3 * time.Second,
		batchCount:    100,
	}

	b.applyOptions()

	// If no pre-configured client, create one from Addrs
	if b.client == nil {
		addr := "localhost:6379"
		if len(options.Addrs) > 0 && options.Addrs[0] != "" {
			addr = options.Addrs[0]
		}
		b.client = redis.NewClient(&redis.Options{
			Addr: addr,
		})
	}

	return b
}

// isGroupExistsError checks if the error is "BUSYGROUP Consumer Group name already exists".
func isGroupExistsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
