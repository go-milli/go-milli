# milli Examples

This `.examples` directory provides sample implementations demonstrating the core features of the `milli` message-driven microservice framework.

## 1. `helloworld`
The simplest possible `Pub/Sub` architecture.
Demonstrates how to use `milli.RegisterSubscriber` to pass strongly typed structs automatically using go generics.

- `cd helloworld && go run main.go`

## 2. `middleware`
Demonstrates how to mount the built-in middlewares like `consumer.LoggingMiddleware` and `consumer.TracingMiddleware` to intercept and decorate the request processing pipeline.

- `cd middleware && go run main.go`

## 3. `manualack`
A demonstration of the manual ACK feature when dealing with hard-core asynchronous database operations or long pooling tasks.
Shows how to turn off native `AutoAck`, use the `consumer.ContextAckWrapper`, and trigger a safe cross-routine `milli.Ack(ctx)`.

- `cd manualack && go run main.go`

## 4. `graceful`
An explicit demonstration showing exactly how the built-in `sync.WaitGroup` prevents your running worker goroutines from being killed instantly when `SIGTERM` (Ctrl+C) is received.

- `cd graceful && go run main.go`
