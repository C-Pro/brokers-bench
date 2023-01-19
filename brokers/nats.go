package brokers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type Nats struct {
	cl *nats.Conn
	js nats.JetStreamContext
}

func NewNats(url, topic string) (*Nats, error) {
	n := &Nats{}
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	n.cl = nc
	n.js = js
	return n, nil
}

func (n *Nats) Produce(ctx context.Context, topic, key, value string) error {
	_, err := n.js.Publish(topic, []byte(value))

	return err
}

func (n *Nats) Consume(ctx context.Context, topic string) (chan Message, error) {
	ch := make(chan Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	sub, err := n.js.SubscribeSync(topic, nats.Durable("test"))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	go func() {
		for {
			m, err := sub.NextMsg(time.Minute)
			if err != nil {
				if errors.Is(err, nats.ErrTimeout) {
					continue
				}
				panic(fmt.Sprintf("consume error: %v", err))
			}

			select {
			case <-ctx.Done():
				return
			case ch <- Message{
				Value: string(m.Data),
			}:
			}

			// if err := m.AckSync(); err != nil {
			// 	panic(fmt.Sprintf("failed to ack: %v", err))
			// }
		}
	}()

	return ch, nil
}
