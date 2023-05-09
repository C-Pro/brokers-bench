package brokers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

type RedPanda struct {
	cl *kgo.Client
	tx bool
}

func NewRedPanda(url, topic string, transactional bool) (*RedPanda, error) {
	rp := &RedPanda{}
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(url, ",")...),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		kgo.ProducerLinger(time.Millisecond),
		kgo.WithLogger(kgo.BasicLogger(os.Stderr, kgo.LogLevelWarn, nil)),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	}

	if topic != "" {
		opts = append(opts, kgo.ConsumeTopics(strings.Split(topic, ",")...))
	}

	if transactional {
		opts = append(opts,
			kgo.TransactionalID(uuid.NewString()),
			kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		)
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	rp.cl = cl
	rp.tx = transactional
	return rp, nil
}

func (rp *RedPanda) Produce(ctx context.Context, topic, key, value string) error {
	msg := kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: []byte(value),
	}

	if rp.tx {
		if err := rp.cl.BeginTransaction(); err != nil {
			return err
		}
	}

	res := rp.cl.ProduceSync(ctx, &msg)

	if rp.tx {
		if res.FirstErr() != nil {
			if err := rp.cl.EndTransaction(ctx, kgo.TryAbort); err != nil {
				return fmt.Errorf("failed to abort tx after %q: %v", res.FirstErr().Error(), err)
			}
		}

		return rp.cl.EndTransaction(ctx, kgo.TryCommit)
	}

	return res.FirstErr()
}

func (rp *RedPanda) Consume(ctx context.Context, topic string) (chan Message, error) {
	ch := make(chan Message)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		wg.Wait()
		close(ch)
	}()

	go func() {
		defer wg.Done()
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
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch, nil
}
