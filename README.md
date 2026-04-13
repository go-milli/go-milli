# Go-Milli 🚀

Go-Milli is an enterprise-grade, message-driven microservice framework in Go. It simplifies building asynchronous and streaming applications by providing powerful abstractions over brokers (like Kafka or In-Memory), strongly-typed Pub/Sub, and an onion-ring middleware architecture.

## 🔥 Key Features

- **Strongly-Typed Handlers**: Generics-based subscribers let you focus on business logic `func(ctx, msg *MyStruct) error` instead of raw byte parsing.
- **Pluggable Broker Abstraction**: Swap between In-Memory (dev) and Kafka (prod) seamlessly.
- **Onion-Ring Middlewares (Wrappers)**: Chainable middleware for both Publishers and Subscribers.
- **Production-Ready Built-ins**:
    - **OpenTelemetry Tracing**: Distributed tracing across message boundaries.
    - **Prometheus Metrics**: Automatic metrics for publish/subscribe rates and latency.
    - **Retry & DLQ**: Exponential backoff retries and Dead Letter Queue fallback.
    - **Structured Logging**: Abstracted logger with high-performance `Zap` implementation.
- **Context Propagation**: Safely propagate custom headers and tracing data across network hops.
- **Load Balancing (Consumer Groups)**: Safely share topics across multiple instances using Service Name as the default Consumer Group.
- **Web Gateway (Gin)**: Effortlessly expose HTTP endpoints that publish messages.

---

## 🏗 Architecture

```text
                    go-milli 
                         
          ┌─── OTel ───┐ ┌── Prometheus ──┐
          │   Tracing   │ │    Metrics     │
          └──────┬──────┘ └───────┬────────┘
                 │                │
    ┌────────────▼────────────────▼──────────┐
    │            Wrapper 中间件洋葱圈           │
    │  Retry → Metrics → OTel → Logging      │
    ├─────────────────────────────────────────┤
    │  Publisher              Consumer        │
    │  (强类型发布)           (强类型订阅)       │
    ├─────────────────────────────────────────┤
    │              Broker 抽象                 │
    │         Memory │ Kafka │ ...            │
    ├─────────────────────────────────────────┤
    │   Logger (Zap)     │  Metadata (Ctx)    │
    ├─────────────────────────────────────────┤
    │          Web (Gin + Publisher)           │
    └─────────────────────────────────────────┘
```

---

## ⚡️ Quick Start

### 1. Initialize the Service
```go
import (
	"github.com/go-milli/go-milli"
	"github.com/go-milli/go-milli/broker/kafka"
	"github.com/go-milli/go-milli/broker"
)

func main() {
	srv := milli.NewService(
		milli.Name("my.service"),
		milli.Broker(kafka.NewBroker(broker.Addrs("127.0.0.1:9092"))),
	)
    // ...
}
```

### 2. Subscribe to a Topic
Go-Milli automatically deserializes messages into your local structs.
```go
type OrderEvent struct {
	OrderID string `json:"order_id"`
}

// Strongly-typed handler!
handler := func(ctx context.Context, msg *OrderEvent) error {
	logger.Infof("Processing order: %s", msg.OrderID)
	return nil // Returning nil automatically ACKs the message
}

milli.RegisterSubscriber("order.events", srv, handler)
```

### 3. Publish Messages
```go
pub := milli.NewEvent("order.events", srv.Publisher())

msg := &OrderEvent{OrderID: "ORD-123"}
if err := pub.Publish(context.Background(), msg); err != nil {
	logger.Errorf("Failed to publish: %v", err)
}
```

### 4. Run the Service
Blocks the main thread, connects the broker, starts consuming, and listens for OS signals for graceful shutdown.
```go
if err := srv.Run(); err != nil {
	logger.Fatal(err)
}
```

---

## 🛠 Advanced Capabilities

### Middlewares (Wrappers)
Go-Milli provides native wrappers to inject powerful behavior. You can chain as many as you need.

#### 1. Tracing (OpenTelemetry)
Automatically propagates traces over message queues:
```go
import "github.com/go-milli/go-milli/wrapper/trace/otel"

srv := milli.NewService(
	// ...
	milli.WrapPublisher(otel.NewPublisherWrapper()),
	milli.WrapSubscriber(otel.NewSubscriberWrapper()),
)
```

#### 2. Metrics (Prometheus)
Records publishing and subscribing durations, and count errors/success rates:
```go
import promwrapper "github.com/go-milli/go-milli/wrapper/metrics/prometheus"

srv := milli.NewService(
	// ...
	milli.WrapPublisher(promwrapper.NewPublisherWrapper()),
	milli.WrapSubscriber(promwrapper.NewSubscriberWrapper()),
)
```

#### 3. Automatic Retries & DLQ
If your handler returns an `error`, the message will automatically be retried. If retries are exhausted, it will be sent to a DLQ (Dead Letter Queue) topic.
```go
import "github.com/go-milli/go-milli/wrapper/retry"

srv := milli.NewService(
	// ...
	milli.WrapSubscriber(
		retry.NewSubscriberWrapper(
			retry.MaxRetries(3),
			retry.InitialBackoff(500 * time.Millisecond),
			retry.DLQ("my.dlq.topic", brokerInstance),
		),
	),
)
```

### Consumer Groups (Load Balancing)
When scaling horizontally, you want your instances to *share* the load of a topic (not broadcast to all).
By default, the **Consumer Group Name equals the Service Name** (`milli.Name()`).

You can override this if a single service needs varying view points of the same topic (e.g., streaming vs archiving):
```go
milli.RegisterSubscriber("order.events", srv, myArchiveHandler, 
	consumer.SubscriberQueue("order.archive.group"),
)
```

### Manual ACKs
By default, Go-Milli Auto-ACKs a message if your handler returns `nil`. If you spawn goroutines inside your handler and exit early, you risk message loss if the goroutine later fails. Use Manual ACKs:

```go
// 1. Subscribe with DisableAutoAck + ContextAckWrapper
milli.RegisterSubscriber("topic", srv, handler, 
    consumer.DisableAutoAck(),
)
srv = milli.NewService( /*...*/ milli.WrapSubscriber(consumer.ContextAckWrapper) )

// 2. In your handler:
handler := func(ctx context.Context, msg *MyMsg) error {
	go func() {
		defer milli.Ack(ctx) // Manually ack when async work completes!
		doHeavyLifting(msg)
	}()
	return nil
}
```

### Web Gateway (Gin Integration)
Go-Milli makes it trivial to spin up an HTTP gateway that publishes messages downstream:

```go
import "github.com/go-milli/go-milli/web"

s := web.NewService(
	web.Name("api.gateway"),
	web.Address(":8080"),
	web.Broker(kb),
	web.WrapPublisher(otel.NewPublisherWrapper()), // Yes, wrappers work here too!
)

r := s.Engine() // Returns *gin.Engine!
r.POST("/api/orders", func(c *gin.Context) {
    // Easily extract context from HTTP request and pass to Publisher
    event := milli.NewEvent("order.topic", s.Publisher())
    event.Publish(c.Request.Context(), payload)
})

s.Run()
```

---

## 📚 Examples Showcase

Check out the `examples/` directory for runable code covering every scenario:
- [`helloworld/`](examples/helloworld) - Basic pub/sub setup using strongly-typed structs
- [`web/`](examples/web) - HTTP API Gateway publishing to Kafka using Gin
- [`trace/`](examples/trace) - OpenTelemetry Distributed Tracing (Span linking across bounds)
- [`middleware/`](examples/middleware) - Custom wrappers / Onion-Ring architecture
- [`manualack/`](examples/manualack) - Safely processing long-running async tasks
- [`retry/`](examples/retry) - Error recovery, Exponential Backoff, DLQ
- [`metrics/`](examples/metrics) - Prometheus scraping for pub/sub rates
- [`consumergroup/`](examples/consumergroup) - Consumer Group load balancing and custom Queues
