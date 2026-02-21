# Go-Milli 🚀

Go-Milli 是一个专为 Go 语言打造的企业级、基于消息驱动的微服务框架。它通过提供对底层 Broker（如 Kafka 或内存队列）的强大抽象、强类型的发布/订阅模型，以及基于洋葱圈模型的中间件架构，极大简化了异步和流式数据处理应用的开发。

## 🔥 核心特性

- **强类型 Handler**：基于泛型设计的订阅者，让你只需关注业务逻辑 `func(ctx, msg *MyStruct) error`，告别繁琐的字节解析。
- **可插拔 Broker 抽象**：在开发环境（内存队列）和生产环境（Kafka）之间无缝切换。
- **洋葱圈中间件 (Wrappers)**：支持为发布者（Publisher）和订阅者（Subscriber）链式挂载中间件。
- **开箱即用的生产级能力**：
    - **OpenTelemetry 链路追踪**：跨越消息队列边界的分布式追踪。
    - **Prometheus 监控指标**：自动采集发布/订阅的吞吐量与耗时。
    - **重试与死信队列 (DLQ)**：支持指数退避的自动重试机制，并在耗尽后将毒丸消息打入 DLQ。
    - **结构化日志**：抽象的日志接口，内置高性能的 `Zap` 实现。
- **Context 传递**：在网络链路中安全、无缝地透传自定义 Header 和 Tracing 数据。
- **消费组与负载均衡**：默认使用 Service Name 作为消费组名，多实例自动实现负载均衡，安全可靠。
- **Web 网关 (Gin)**：轻松启动 HTTP 网关并向下游发布消息。

---

## 🏗 架构全景

```text
                    go-milli 
                         
          ┌─── OTel ───┐ ┌── Prometheus ──┐
          │ 引流与追踪   │ │   监控与告警     │
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

## ⚡️ 快速开始

### 1. 初始化服务
```go
import (
	"github.com/aevumio/go-milli"
	"github.com/aevumio/go-milli/broker/kafka"
	"github.com/aevumio/go-milli/broker"
)

func main() {
	srv := milli.NewService(
		milli.Name("my.service"),
		milli.Broker(kafka.NewBroker(broker.Addrs("127.0.0.1:9092"))),
	)
    // ...
}
```

### 2. 订阅主题
Go-Milli 会自动将消息反序列化为你定义的本地结构体。
```go
type OrderEvent struct {
	OrderID string `json:"order_id"`
}

// 强类型的业务 Handler！
handler := func(ctx context.Context, msg *OrderEvent) error {
	logger.Infof("正在处理订单: %s", msg.OrderID)
	return nil // 返回 nil 框架会自动发送 ACK 给 Broker
}

milli.RegisterSubscriber("order.events", srv, handler)
```

### 3. 发布消息
```go
pub := milli.NewEvent("order.events", srv.Publisher())

msg := &OrderEvent{OrderID: "ORD-123"}
if err := pub.Publish(context.Background(), msg); err != nil {
	logger.Errorf("消息发送失败: %v", err)
}
```

### 4. 运行服务
阻塞主线程，连接 Broker，启动消费协程，并监听系统信号以实现优雅退出。
```go
if err := srv.Run(); err != nil {
	logger.Fatal(err)
}
```

---

## 🛠 企业级高级能力

### 中间件 (Wrappers)
Go-Milli 提供了原生的 Wrapper 接口，用于注入强大的拦截器行为。你可以按需链式挂载多个 Wrapper。

#### 1. 链路追踪 (OpenTelemetry)
自动提取和注入 Context，跨越消息队列传播分布式追踪的 TraceID：
```go
import "github.com/aevumio/go-milli/wrapper/trace/otel"

srv := milli.NewService(
	// ...
	milli.WrapPublisher(otel.NewPublisherWrapper()),
	milli.WrapSubscriber(otel.NewSubscriberWrapper()),
)
```

#### 2. 监控告警 (Prometheus)
自动记录消息处理和发送的持续时间、错误率以及吞吐量：
```go
import promwrapper "github.com/aevumio/go-milli/wrapper/metrics/prometheus"

srv := milli.NewService(
	// ...
	milli.WrapPublisher(promwrapper.NewPublisherWrapper()),
	milli.WrapSubscriber(promwrapper.NewSubscriberWrapper()),
)
```

#### 3. 自动重试与死信队列 (DLQ)
当 Handler 返回 `error` 时，中间件会自动进行指数退避重试。如果重试次数耗尽，消息将被安全地转发至 DLQ（死信队列）主题，防止阻塞正常消费。
```go
import "github.com/aevumio/go-milli/wrapper/retry"

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

### 消费组 (Consumer Groups) 与负载均衡
在水平扩容时，你希望多个实例**分摊**同一个 Topic 的消息（而不是广播）。
默认情况下，Go-Milli 使用 **Service Name 作为消费组名称** (`milli.Name()`)。

如果同一个服务需要以不同视角消费同一个 Topic（例如：一路实时处理，另一路归档），你可以覆盖默认配置：
```go
milli.RegisterSubscriber("order.events", srv, myArchiveHandler, 
	consumer.SubscriberQueue("order.archive.group"), // 覆盖默认消费组
)
```

### 手动确认 (Manual ACKs)
默认情况下，如果你的 Handler 返回 `nil`，框架会自动确认 (Auto-ACK) 消息。如果你的 Handler 会启动异步协程并立即返回，且该协程后续发生崩溃，你就会丢失消息。在此类场景下，请使用手动确认：

```go
// 1. 关闭 AutoAck，并挂载 ContextAck 注入器
milli.RegisterSubscriber("topic", srv, handler, 
    consumer.DisableAutoAck(),
)
srv = milli.NewService( /*...*/ milli.WrapSubscriber(consumer.ContextAckWrapper) )

// 2. 在业务 Handler 中掏出 Context 里的闭包并触发：
handler := func(ctx context.Context, msg *MyMsg) error {
	go func() {
		defer milli.Ack(ctx) // 异步任务完成后，安全地执行手动 Ack！
		doHeavyLifting(msg)
	}()
	return nil
}
```

### Web 网关集成 (Gin Integration)
Go-Milli 可以一键拉起 HTTP 网关，并在 HTTP 路由中向下游消息队列发送带 Context 的消息：

```go
import "github.com/aevumio/go-milli/web"

s := web.NewService(
	web.Name("api.gateway"),
	web.Address(":8080"),
	web.Broker(kb),
	web.WrapPublisher(otel.NewPublisherWrapper()), // Web 服务同样支持 Wrappers！
)

r := s.Engine() // 直接获取原生 *gin.Engine!
r.POST("/api/orders", func(c *gin.Context) {
    // 轻松提取 HTTP Request 的 Context 并传给 Publisher
    event := milli.NewEvent("order.topic", s.Publisher())
    event.Publish(c.Request.Context(), payload)
})

s.Run()
```

---

## 📚 示例代码库 (Examples Showcase)

查看 `examples/` 目录获取涵盖各个场景的可运行示例代码：
- [`helloworld/`](examples/helloworld) - 使用强类型结构体的基础 Pub/Sub 演示
- [`web/`](examples/web) - 使用 Gin 构建 HTTP API 网关并向 Kafka 发布消息
- [`trace/`](examples/trace) - OpenTelemetry 分布式链路追踪（跨消息边界连接 Span）
- [`middleware/`](examples/middleware) - 自定义 Wrapper 拦截器与洋葱圈架构
- [`manualack/`](examples/manualack) - 如何安全地处理耗时异步任务并手动 Ack 防止丢消息
- [`retry/`](examples/retry) - 错误恢复、指数退避重试机制与 DLQ 死信队列
- [`metrics/`](examples/metrics) - Prometheus 指标采集与 HTTP Endpoint 暴露
- [`consumergroup/`](examples/consumergroup) - 多个消费组负载均衡与多视角消费演示
