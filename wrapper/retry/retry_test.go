package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-milli/go-milli/broker"
	"github.com/go-milli/go-milli/consumer"
)

func TestFailedDLQPublishReturnsError(t *testing.T) {
	b := broker.NewMemoryBroker() // Deliberately disconnected: publishing must fail.
	handlerErr := errors.New("delivery failed")
	h := NewSubscriberWrapper(MaxRetries(0), DLQ("dead", b))(func(context.Context, consumer.Message) error {
		return handlerErr
	})
	err := h(context.Background(), consumer.Message{Topic: "orders", Header: map[string]string{}})
	if err == nil {
		t.Fatal("failed DLQ publish was reported as success; original message would be acknowledged")
	}
	if !errors.Is(err, handlerErr) {
		t.Fatalf("original handler error lost: %v", err)
	}
}

func TestSuccessfulDLQPreservesOriginalMessage(t *testing.T) {
	for _, headers := range []map[string]string{nil, {"trace": "123"}} {
		t.Run("headers", func(t *testing.T) {
			b := broker.NewMemoryBroker()
			if err := b.Connect(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = b.Disconnect() })
			var received *broker.Message
			sub, err := b.Subscribe("dead", func(e broker.Event) error { received = e.Message(); return nil })
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sub.Unsubscribe() })
			h := NewSubscriberWrapper(MaxRetries(0), DLQ("dead", b))(func(context.Context, consumer.Message) error {
				return errors.New("delivery failed")
			})
			if err := h(context.Background(), consumer.Message{Topic: "orders", Header: headers, Body: []byte("payload")}); err != nil {
				t.Fatal(err)
			}
			if received == nil || string(received.Body) != "payload" || received.Header["X-Original-Topic"] != "orders" || received.Header["X-Last-Error"] != "delivery failed" {
				t.Fatalf("incorrect dead letter: %+v", received)
			}
			if _, ok := headers["X-Original-Topic"]; ok {
				t.Fatal("DLQ modified original message headers")
			}
		})
	}
}

func TestCancellationInterruptsRetryBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempted := make(chan struct{})
	attempts := 0
	h := NewSubscriberWrapper(MaxRetries(1), InitialBackoff(time.Second))(func(context.Context, consumer.Message) error {
		attempts++
		if attempts == 1 {
			close(attempted)
		}
		return errors.New("retry me")
	})
	done := make(chan error, 1)
	go func() { done <- h(ctx, consumer.Message{}) }()
	<-attempted
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
		if attempts != 1 {
			t.Fatalf("handler called %d times after cancellation", attempts)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("retry did not stop when context was canceled")
		<-done
	}
}
