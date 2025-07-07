// Package apikeys for sending apikeys usage to amqp broker.
package apikeys

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/pkg/apikeys"
)

// Broker contains amqp channel.
type Broker struct {
	Channel *amqp.Channel
}

// SendAPIKeyUsageLog send apikeyID reason and source ip to broker.
func (b *Broker) SendAPIKeyUsageLog(apikeyID string, reason apikeys.UsageReason, fromIP string) error {
	a := &apikeys.APIKeyUsage{
		ApikeyId: apikeyID,
		Reason:   reason,
		FromIP:   fromIP,
	}
	data, err := proto.Marshal(a)
	if err != nil {
		return fmt.Errorf("can`t marshal record: %w", err)
	}

	err = b.Channel.Publish(
		config.APIKeyUsageExchange,
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/protobuf",
			Body:        data,
		},
	)
	if err != nil {
		return fmt.Errorf("can`t publish api_key_usage log: %w", err)
	}

	return nil
}

// InitBroker creates connection to amqp broker.
func InitBroker(connectURL string) (*Broker, error) {
	rabbitmqcon, err := amqp.Dial(connectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := rabbitmqcon.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		config.APIKeyUsageExchange,
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq exchange '%s': %w", config.APIKeyUsageExchange, err)
	}

	return &Broker{
		Channel: ch,
	}, nil
}
