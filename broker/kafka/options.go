package kafka

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/go-milli/go-milli/broker"
)

var (
	DefaultBrokerConfig  = sarama.NewConfig()
	DefaultClusterConfig = sarama.NewConfig()
)

type brokerConfigKey struct{}
type clusterConfigKey struct{}

func BrokerConfig(c *sarama.Config) broker.Option {
	return setBrokerOption(brokerConfigKey{}, c)
}

func ClusterConfig(c *sarama.Config) broker.Option {
	return setBrokerOption(clusterConfigKey{}, c)
}

type asyncProduceErrorKey struct{}
type asyncProduceSuccessKey struct{}

func AsyncProducer(errors chan<- *sarama.ProducerError, successes chan<- *sarama.ProducerMessage) broker.Option {
	// set default opt
	var opt = func(options *broker.Options) {}
	if successes != nil {
		opt = setBrokerOption(asyncProduceSuccessKey{}, successes)
	}
	if errors != nil {
		opt = setBrokerOption(asyncProduceErrorKey{}, errors)
	}
	return opt
}

type subscribeContextKey struct{}

// SubscribeContext set the context for broker.SubscribeOption.
func SubscribeContext(ctx context.Context) broker.SubscribeOption {
	return setSubscribeOption(subscribeContextKey{}, ctx)
}

type subscribeConfigKey struct{}

func SubscribeConfig(c *sarama.Config) broker.SubscribeOption {
	return setSubscribeOption(subscribeConfigKey{}, c)
}
