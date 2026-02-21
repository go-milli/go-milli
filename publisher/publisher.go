package publisher

import (
	"context"
)

// Publisher 接口，作为发布引擎的总入口
type Publisher interface {
	Options() Options
	Publish(ctx context.Context, msg Message, opts ...PublishOption) error
	String() string
}

// Wrapper is a middleware interface for publishing messages
type Wrapper func(Publisher) Publisher

var (
	// DefaultPublisher is the default publisher
	DefaultPublisher Publisher = NewDefaultPublisher()
)

// Publish default publisher
func Publish(ctx context.Context, msg Message, opts ...PublishOption) error {
	return DefaultPublisher.Publish(ctx, msg, opts...)
}

// String returns the name of the publisher
func String() string {
	return DefaultPublisher.String()
}
