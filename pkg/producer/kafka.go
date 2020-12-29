package producer

import (
	"context"
	"errors"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// KafkaProducer is a Producer implementation for sending events through a Kafka topic
type KafkaProducer struct {
	writer  *kafka.Writer
	timeout time.Duration
}

// NewKafkaProducer checks the provided addresses and creates a Kafka producer
func NewKafkaProducer(conf Config) (*KafkaProducer, error) {
	if !oneUp(conf.Addresses, conf.Timeout) {
		return nil, errors.New("none of the provided Kafka broker endpoints are usable")
	}
	producer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  conf.Addresses,
		Topic:    conf.Topic,
		Balancer: kafka.Murmur2Balancer{},
	})
	return &KafkaProducer{writer: producer, timeout: conf.Timeout}, nil
}

// Close shuts down the underlying Kafka writer
func (kp *KafkaProducer) Close() {
	kp.writer.Close()
}

// Send writes the given event to the configured Kafka topic
func (kp *KafkaProducer) Send(seriesID string, event []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), kp.timeout)
	defer cancel()
	return kp.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(seriesID),
		Value: event,
	})
}
