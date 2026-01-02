package kafka

import (
	"context"
	"log"

	"github.com/IBM/sarama"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type HandlerFunc func(ctx context.Context, msg *sarama.ConsumerMessage) error

type ConsumerGroup struct {
	brokers     []string
	groupID     string
	topics      []string
	handlerFunc HandlerFunc
	logger      *zap.Logger
}

func NewConsumerGroup(
	brokers []string,
	groupID string,
	topics []string,
	handlerFunc HandlerFunc,
	logger *zap.Logger,
) *ConsumerGroup {
	return &ConsumerGroup{
		brokers:     brokers,
		groupID:     groupID,
		topics:      topics,
		handlerFunc: handlerFunc,
		logger:      logger,
	}
}

func (c *ConsumerGroup) Run(ctx context.Context) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.BalanceStrategyRoundRobin}

	group, err := sarama.NewConsumerGroup(c.brokers, c.groupID, config)
	if err != nil {
		log.Fatalf("Error creating new consumer group")
	}

	defer func() {
		if err := group.Close(); err != nil {
			log.Fatalf("Error closing consumer group: %v", err)
		}
	}()

	consumer := &saramaHandler{
		handler: c.handlerFunc,
		logger:  c.logger,
	}

	for {
		err := group.Consume(ctx, c.topics, consumer)
		if err != nil {
			mylogger.Error(ctx, c.logger, "Error consuming in consumer loop", zap.Error(err))
		}

		if ctx.Err() != nil {
			mylogger.Info(ctx, c.logger, "Context cancelled, shutting down consumer")
			return
		}
	}
}

type saramaHandler struct {
	handler HandlerFunc
	logger  *zap.Logger
}

func (h *saramaHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *saramaHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *saramaHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		ctx := h.extractTracing(session.Context(), msg)

		err := h.handler(ctx, msg)
		if err == nil {
			session.MarkMessage(msg, "")
		} else {
			mylogger.Error(
				ctx,
				h.logger,
				"Failed to process message",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (h *saramaHandler) extractTracing(ctx context.Context, msg *sarama.ConsumerMessage) context.Context {
	carrier := propagation.MapCarrier{}
	for _, header := range msg.Headers {
		carrier[string(header.Key)] = string(header.Value)
	}

	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, carrier)

	tracer := otel.Tracer("pkg/kafka/consumer")
	ctx, _ = tracer.Start(ctx, "kafka_process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(),
	)
	return ctx
}
