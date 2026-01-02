package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Producer interface {
	ProduceMessage(ctx context.Context, topic string, message interface{}) error
	Close() error
}

type producer struct {
	syncProducer sarama.SyncProducer
}

func NewProducer(brokers []string) (Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	p, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("error creating producer: %v", err)
	}

	return &producer{syncProducer: p}, nil
}

func (p *producer) ProduceMessage(ctx context.Context, topic string, message interface{}) error {
	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return err
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		log.Printf("✅ Context has TraceID: %s", span.SpanContext().TraceID().String())
	} else {
		log.Printf("❌ Context has NO TraceID (Span is invalid)")
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	if len(carrier) == 0 {
		log.Printf("❌ Carrier is EMPTY after Inject! (Propagator not set?)")
	} else {
		for k, v := range carrier {
			log.Printf("✅ Header to send: %s = %s", k, v)
		}
	}

	var headers []sarama.RecordHeader
	for k, v := range carrier {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Value:   sarama.StringEncoder(jsonMsg),
		Headers: headers,
	}

	partition, offset, err := p.syncProducer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}

	log.Printf("✅ Message sent to topic %s (Partition: %d, Offset: %d)\n", topic, partition, offset)
	return nil
}

func (p *producer) Close() error {
	return p.syncProducer.Close()
}
