package milli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/go-milli/go-milli/consumer"
	"github.com/go-milli/go-milli/logger"
	"github.com/go-milli/go-milli/publisher"
)

type service struct {
	opts Options
	c    consumer.Consumer
	p    publisher.Publisher
}

// NewService creates and returns a new Service
func NewService(opts ...Option) Service {
	options := newOptions(opts...)

	c := options.Consumer
	if c == nil {
		c = consumer.NewDefaultConsumer(
			consumer.Name(options.Name),
			consumer.Broker(options.Broker),
			consumer.Codec(options.Codec),
			consumer.WrapSubscriber(options.SubscriberWrappers...),
		)
	}

	p := options.Publisher
	if p == nil {
		p = publisher.NewDefaultPublisher(
			publisher.Name(options.Name),
			publisher.Broker(options.Broker),
			publisher.Codec(options.Codec),
			publisher.WrapPublisher(options.PublisherWrappers...),
		)
	}

	return &service{
		opts: options,
		c:    c,
		p:    p,
	}
}

func (s *service) Name() string {
	return s.opts.Name
}

func (s *service) Consumer() consumer.Consumer {
	return s.c
}

func (s *service) Publisher() publisher.Publisher {
	return s.p
}

func (s *service) Run() error {
	logger.DefaultLogger.Logf(logger.InfoLevel, "Starting [service] %s", s.Name())

	// 启动 consumer，它连接 Broker 并注册所有 handler
	if err := s.c.Start(); err != nil {
		return err
	}

	// 阻塞等待系统中断信号
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	sig := <-ch

	logger.DefaultLogger.Logf(logger.InfoLevel, "Received signal %s", sig)

	// 后处理，优雅停止
	logger.DefaultLogger.Logf(logger.InfoLevel, "Stopping [service] %s", s.Name())
	return s.c.Stop()
}
