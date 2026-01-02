package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/sakashimaa/go-pet-project/notification/internal/domain"
	"github.com/sakashimaa/go-pet-project/notification/internal/service"
	"github.com/sakashimaa/go-pet-project/pkg/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.uber.org/zap"
)

type Consumer struct {
	service *service.NotificationService
	logger  *zap.Logger
}

func NewConsumer(service *service.NotificationService, logger *zap.Logger) *Consumer {
	return &Consumer{
		service: service,
		logger:  logger,
	}
}

func (c *Consumer) Start(ctx context.Context, brokers []string) {
	consumerGroup := kafka.NewConsumerGroup(
		brokers,
		"notification-service-group",
		[]string{"user_events"},
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
			log.Printf("❌ Error parsing event: %v", err)
			return nil
		}

		if err := c.service.HandleUserRegistered(ctx, event); err != nil {
			log.Printf("❌ Error processing register event: %v", err)
			return err
		}
	case "UserForgotPassword":
		var event domain.UserForgotPasswordEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("❌ Error parsing event: %v", err)
			return nil
		}

		if err := c.service.HandleUserForgotPassword(ctx, event); err != nil {
			log.Printf("❌ Error processing forgot password event: %v", err)
			return err
		}
	case "UserResetPassword":
		var event domain.UserResetPasswordEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("❌ Error parsing event: %v", err)
			return nil
		}

		if err := c.service.HandleUserResetPassword(ctx, event); err != nil {
			log.Printf("❌ Error processing reset password event: %v", err)
			return err
		}
	default:
		log.Printf("Ignored event type: %s", wrapper.Event)
	}

	return nil
}
