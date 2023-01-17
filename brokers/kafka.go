package brokers

import (
	"context"
	"log"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type Kafka struct {
	writer *kafka.Writer
	reader *kafka.Reader
}

func NewKafka(url, topic string) *Kafka {
	k := Kafka{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(url),
			Topic:                  topic,
			AllowAutoTopicCreation: true,
			// Async:                  true, // NO-NO
			// BatchSize:              10,
			// BatchBytes:             1024 * 64,
			BatchTimeout: time.Millisecond * 100,
		},
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{url},
			GroupID: "1",
			Topic:   topic,
			// MinBytes:    10e3, // 10KB
			// MaxBytes:    10e6, // 10MB
			StartOffset: kafka.LastOffset,
			MaxWait:     time.Millisecond,
		}),
	}

	return &k
}

func (k *Kafka) Produce(ctx context.Context, topic, key, value string) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: []byte(value),
	}

	return k.writer.WriteMessages(ctx, msg)
}

func (k *Kafka) Consume(ctx context.Context, topic string) (chan Message, error) {
	ch := make(chan Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	go func() {
		for {
			m, err := k.reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("reader returned %v", err)
				return
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
