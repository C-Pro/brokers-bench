package brokers

import (
	"context"
	"errors"
	"log"
	"strings"

	kafka "github.com/segmentio/kafka-go"
)

type Kafka struct {
	writer *kafka.Writer
	reader *kafka.Reader
}

func NewKafka(url, topic string) *Kafka {
	urls := strings.Split(url, ",")
	k := Kafka{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(urls...),
			BatchSize:    1,
			RequiredAcks: kafka.RequireOne,
			Balancer:     &kafka.RoundRobin{},
			Compression:  kafka.Snappy,
		},
	}

	if topic != "" {
		k.reader = kafka.NewReader(kafka.ReaderConfig{
			Brokers: urls,
			Topic:   topic,
		})
		k.reader.SetOffset(kafka.LastOffset)
	}

	return &k
}

func (k *Kafka) Produce(ctx context.Context, topic, key, value string) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: []byte(value),
	}

	return k.writer.WriteMessages(ctx, msg)
}

func (k *Kafka) Consume(ctx context.Context, topic string) (chan Message, error) {
	if k.reader == nil {
		return nil, errors.New("not created as a consumer (no topic provided)")
	}
	ch := make(chan Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	go func() {
		for {
			m, err := k.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("reader returned %v", err)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- Message{
				Key:       string(m.Key),
				Value:     string(m.Value),
				Timestamp: &m.Time,
			}:
			}
		}
	}()

	return ch, nil
}
