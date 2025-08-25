// Package eventhandler contains event handler that sends messages to rabbitmq.
package eventhandler

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

// RabbitMQEventHandler implementation of EventHandler. Sends messages to rabbitmq.
type RabbitMQEventHandler struct {
	channel *amqp.Channel
}

// NewRabbitMQEventHandler constructor for RabbitMQEventHandler.
func NewRabbitMQEventHandler(
	channel *amqp.Channel,
) RabbitMQEventHandler {
	return RabbitMQEventHandler{
		channel: channel,
	}
}

// Notify implementation of abstract method EventHandler.Notify.
func (h RabbitMQEventHandler) Notify(ev event.Event) {
	switch e := ev.(type) {
	case event.APIKeyUsedEvent:
		_ = h.sendAPIKeyUsageLog(e.APIKeyID(), e.Reason(), e.FromIP())
	}
}

func (h *RabbitMQEventHandler) sendAPIKeyUsageLog(apikeyID string, reason apikeys.UsageReason, fromIP string) error {
	a := &apikeys.APIKeyUsage{
		ApikeyId: apikeyID,
		Reason:   reason,
		FromIP:   fromIP,
	}
	data, err := proto.Marshal(a)
	if err != nil {
		return fmt.Errorf("can`t marshal record: %w", err)
	}

	err = h.channel.Publish(
		"apikeysusage",
		"apikeysusage.msg", // routing key
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType: "application/protobuf",
			Body:        data,
		},
	)
	if err != nil {
		return fmt.Errorf("can`t publish apikeyusage log: %w", err)
	}

	return nil
}
