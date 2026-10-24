package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/consumer"
	redisclient "github.com/redis/go-redis/v9"
)

func newTestBroker(t *testing.T) (*miniredis.Miniredis, *redisclient.Client, broker.Broker) {
	t.Helper()
	m := miniredis.RunT(t)
	m.SetTime(time.Unix(1700000000, 0))
	c := redisclient.NewClient(&redisclient.Options{Addr: m.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	b := NewBroker(WithRedisClient(c), WithConsumerGroup("workers"), WithBlockTimeout(10*time.Millisecond))
	if err := b.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("disconnect panicked: %v", r)
			}
		}()
		if err := b.Disconnect(); err != nil {
			t.Error(err)
		}
	})
	return m, c, b
}

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if f() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("condition not met before timeout")
		case <-ticker.C:
		}
	}
}

func receive(t *testing.T, ch <-chan broker.Event) broker.Event {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(4 * time.Second):
		t.Fatal("message was not delivered")
		return nil
	}
}

func pendingCount(c *redisclient.Client) int64 {
	p, err := c.XPending(context.Background(), "events", "workers").Result()
	if err != nil {
		return -1
	}
	return p.Count
}

func TestDefaultAutoAckAndManualAck(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "manual"}[manual], func(t *testing.T) {
			_, c, b := newTestBroker(t)
			deliveries := make(chan broker.Event, 1)
			var opts []broker.SubscribeOption
			if manual {
				opts = append(opts, broker.DisableAutoAck())
			}
			_, err := b.Subscribe("events", func(e broker.Event) error { deliveries <- e; return nil }, opts...)
			if err != nil {
				t.Fatal(err)
			}
			if err := b.Publish("events", &broker.Message{Body: []byte("paid")}); err != nil {
				t.Fatal(err)
			}
			e := receive(t, deliveries)
			if manual {
				if got := pendingCount(c); got != 1 {
					t.Fatalf("manual message acknowledged prematurely: pending=%d", got)
				}
				if err := e.Ack(); err != nil {
					t.Fatal(err)
				}
			}
			eventually(t, func() bool { return pendingCount(c) == 0 })
		})
	}
}

func TestFailedHandlerIsRedelivered(t *testing.T) {
	m, c, b := newTestBroker(t)
	failed := make(chan struct{})
	delivered := make(chan broker.Event, 1)
	var attempts atomic.Int32
	_, err := b.Subscribe("events", func(e broker.Event) error {
		if attempts.Add(1) == 1 {
			close(failed)
			return errors.New("temporary failure")
		}
		delivered <- e
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Publish("events", &broker.Message{Body: []byte("retry")}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-failed:
	case <-time.After(4 * time.Second):
		t.Fatal("first delivery missing")
	}
	m.SetTime(time.Unix(1700000120, 0))
	e := receive(t, delivered)
	if string(e.Message().Body) != "retry" {
		t.Fatal("recovered wrong payload")
	}
	eventually(t, func() bool { return pendingCount(c) == 0 })
}

func TestNewConsumerRecoversAbandonedPending(t *testing.T) {
	m, c, b := newTestBroker(t)
	ctx := context.Background()
	if err := c.XGroupCreateMkStream(ctx, "events", "workers", "0").Err(); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish("events", &broker.Message{Header: map[string]string{"trace": "abc"}, Body: []byte("abandoned")}); err != nil {
		t.Fatal(err)
	}
	if err := c.XReadGroup(ctx, &redisclient.XReadGroupArgs{Group: "workers", Consumer: "dead-worker", Streams: []string{"events", ">"}, Count: 1}).Err(); err != nil {
		t.Fatal(err)
	}
	m.SetTime(time.Unix(1700000120, 0))
	delivered := make(chan broker.Event, 1)
	_, err := b.Subscribe("events", func(e broker.Event) error { delivered <- e; return nil })
	if err != nil {
		t.Fatal(err)
	}
	e := receive(t, delivered)
	if string(e.Message().Body) != "abandoned" || e.Message().Header["trace"] != "abc" {
		t.Fatal("recovery lost message contents")
	}
	eventually(t, func() bool { return pendingCount(c) == 0 })
}

func TestRecentPendingIsNotStolen(t *testing.T) {
	_, c, b := newTestBroker(t)
	ctx := context.Background()
	if err := c.XGroupCreateMkStream(ctx, "events", "workers", "0").Err(); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish("events", &broker.Message{Body: []byte("in-progress")}); err != nil {
		t.Fatal(err)
	}
	if err := c.XReadGroup(ctx, &redisclient.XReadGroupArgs{Group: "workers", Consumer: "active-worker", Streams: []string{"events", ">"}, Count: 1}).Err(); err != nil {
		t.Fatal(err)
	}
	delivered := make(chan broker.Event, 2)
	_, err := b.Subscribe("events", func(e broker.Event) error { delivered <- e; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Publish("events", &broker.Message{Body: []byte("new")}); err != nil {
		t.Fatal(err)
	}
	if got := string(receive(t, delivered).Message().Body); got != "new" {
		t.Fatalf("stole active message: %s", got)
	}
	eventually(t, func() bool { return pendingCount(c) == 1 })
}

func TestConsumerStopAfterUnsubscribe(t *testing.T) {
	_, _, b := newTestBroker(t)
	c := consumer.NewDefaultConsumer(consumer.Name("workers"), consumer.Broker(b))
	if err := c.Subscribe("events", func(context.Context, consumer.Message) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := b.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentUnsubscribeAndDisconnect(t *testing.T) {
	_, _, b := newTestBroker(t)
	s, err := b.Subscribe("events", func(broker.Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("concurrent close panicked: %v", r)
				}
			}()
			if err := s.Unsubscribe(); err != nil {
				t.Error(err)
			}
			if err := b.Disconnect(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
