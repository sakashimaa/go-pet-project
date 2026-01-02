package kafka

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/sakashimaa/go-pet-project/order/internal/domain"
	"github.com/sakashimaa/go-pet-project/order/internal/service"
	generalDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.uber.org/zap"
)

type Consumer struct {
	service service.OrderService
	logger  *zap.Logger
}

func NewConsumer(service service.OrderService, logger *zap.Logger) *Consumer {
	return &Consumer{
		service: service,
		logger:  logger,
	}
}

func (c *Consumer) Start(ctx context.Context, brokers []string) {
	consumerGroup := kafka.NewConsumerGroup(
		brokers,
		"order-service-group-v2",
		[]string{"order_events", "user_events", "payment_events"},
		c.processMessage,
		c.logger,
	)

	consumerGroup.Run(ctx)
}

func (c *Consumer) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	mylogger.Info(
		ctx,
		c.logger,
		"Processing message",
		zap.String("topic", msg.Topic),
	)

	type EventWrapper struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}

	var wrapper EventWrapper
	if err := json.Unmarshal(msg.Value, &wrapper); err != nil {
		mylogger.Error(ctx, c.logger, "Error unmarshalling wrapper", zap.Error(err))
		return err
	}

	switch wrapper.Event {
	case "UserRegistered":
		var event domain.UserRegisteredEvent
		if err := json.Unmarshal(wrapper.Payload, &event); err != nil {
			mylogger.Error(ctx, c.logger, "Failed to unmarshal event", zap.Error(err))
			return err
		}

		err := c.service.HandleUserRegistered(ctx, &event)
		if err != nil {
			mylogger.Error(ctx, c.logger, "Failed to handle register event", zap.Error(err))
			return err
		}

		return nil
	case "PaymentSucceeded":
		var event generalDomain.PaymentSucceededEvent
		if err := json.Unmarshal(wrapper.Payload, &event); err != nil {
			mylogger.Error(ctx, c.logger, "Failed to unmarshal payload", zap.Error(err))
			return err
		}

		err := c.service.ChangeOrderStatus(ctx, &event)
		if err != nil {
			mylogger.Error(ctx, c.logger, "Failed to change order status", zap.Error(err))
			return err
		}
	case "PaymentFailed":
		var event generalDomain.PaymentFailedEvent
		if err := json.Unmarshal(wrapper.Payload, &event); err != nil {
			mylogger.Error(ctx, c.logger, "Failed to unmarshal payload", zap.Error(err))
			return err
		}

		err := c.service.CancelOrder(ctx, &event)
		if err != nil {
			mylogger.Error(ctx, c.logger, "Failed to cancel order", zap.Error(err))
			return err
		}
	default:
		mylogger.Warn(ctx, c.logger, "Ignored event type", zap.String("event_type", wrapper.Event))
	}

	return nil
}
