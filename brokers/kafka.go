package brokers

import (
	"context"
	"log"
	"strings"
	"time"

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
			Addr:                   kafka.TCP(urls...),
			Topic:                  topic,
			AllowAutoTopicCreation: true,
			// Async:                  true, // NO-NO
			// BatchSize:    10,
			// BatchBytes:   1024 * 10,
			BatchTimeout: time.Millisecond,
			RequiredAcks: kafka.RequireAll,
			Balancer: &kafka.RoundRobin{},
		},
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: urls,
			// GroupID:       "999",
			Topic: topic,
			// QueueCapacity: 1,
			StartOffset: kafka.LastOffset,
			// MaxWait:       time.Millisecond,
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
