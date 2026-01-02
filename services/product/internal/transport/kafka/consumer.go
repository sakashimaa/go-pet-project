package kafka

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/kafka"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/service"
	"go.uber.org/zap"
)

type Consumer struct {
	service service.ProductService
	logger  *zap.Logger
}

func NewConsumer(service service.ProductService, logger *zap.Logger) *Consumer {
	return &Consumer{
		service: service,
		logger:  logger,
	}
}

func (c *Consumer) Start(ctx context.Context, brokers []string) {
	consumerGroup := kafka.NewConsumerGroup(
		brokers,
		"product-service-group",
		[]string{"product_events", "order_events"},
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
	case "OrderCreated":
		var event domain.OrderCreatedEvent
		if err := json.Unmarshal(wrapper.Payload, &event); err != nil {
			mylogger.Warn(ctx, c.logger, "Error unmarshalling event structure", zap.Error(err))
			return err
		}

		if err := c.service.ReserveProduct(ctx, &event); err != nil {
			mylogger.Warn(ctx, c.logger, "Error processing order created", zap.Error(err))
			return err
		}
	case "OrderCancelled":
		var event outboxDomain.OrderCancelledEvent
		if err := json.Unmarshal(wrapper.Payload, &event); err != nil {
			mylogger.Warn(ctx, c.logger, "Error unmarshalling event structure", zap.Error(err))
			return err
		}

		if err := c.service.ReturnStock(ctx, &event); err != nil {
			mylogger.Warn(ctx, c.logger, "Error processing return stock", zap.Error(err))
			return err
		}
	default:
		mylogger.Warn(ctx, c.logger, "Ignored event type", zap.String("event_type", wrapper.Event))
	}

	return nil
}
