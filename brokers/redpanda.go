package brokers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
)

type RedPanda struct {
	cl *kgo.Client
}

func NewRedPanda(url, topic string) (*RedPanda, error) {
	rp := &RedPanda{}
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(url, ",")...),
		kgo.DefaultProduceTopic(topic),
		kgo.ConsumeTopics(topic),
		// kgo.MaxConcurrentFetches(1),
		kgo.ProducerBatchMaxBytes(1024 * 1024),
		// kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.DisableIdempotentWrite(),
		// kgo.RequiredAcks(kgo.LeaderAck()),
		kgo.RequiredAcks(kgo.NoAck()),
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelWarn, nil)),
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	rp.cl = cl
	return rp, nil
}

func (rp *RedPanda) Produce(ctx context.Context, topic, key, value string) error {
	msg := kgo.Record{
		Key:   []byte(key),
		Value: []byte(value),
	}

	res := rp.cl.ProduceSync(ctx, &msg)

	return res.FirstErr()
}

func (rp *RedPanda) Consume(ctx context.Context, topic string) (chan Message, error) {
	ch := make(chan Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	go func() {
		for {
			fetches := rp.cl.PollFetches(ctx)
			fetches.EachError(func(t string, p int32, err error) {
				if errors.Is(err, context.Canceled) {
					return
				}
				panic(fmt.Sprintf("topic %s partition %d had error: %v", t, p, err))
			})

			fetches.EachRecord(func(m *kgo.Record) {
				select {
				case <-ctx.Done():
					return
				case ch <- Message{
					Key:       string(m.Key),
					Value:     string(m.Value),
					Timestamp: &m.Timestamp,
				}:
				}
			})
		}
	}()

	return ch, nil
}
