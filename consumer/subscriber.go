package consumer

// subscriber represents a topic subscription
type subscriber struct {
	topic   string
	handler Handler
	opts    SubscriberOptions
}

func newSubscriber(topic string, handler Handler, opts ...SubscriberOption) Subscriber {
	options := SubscriberOptions{
		AutoAck: true,
	}

	for _, o := range opts {
		o(&options)
	}

	return &subscriber{
		topic:   topic,
		handler: handler,
		opts:    options,
	}
}

func (s *subscriber) Topic() string {
	return s.topic
}

func (s *subscriber) Handler() Handler {
	return s.handler
}

func (s *subscriber) Options() SubscriberOptions {
	return s.opts
}
