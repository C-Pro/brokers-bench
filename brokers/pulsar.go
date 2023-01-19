package brokers

import (
	"context"
	"errors"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
)

type Pulsar struct {
	cl pulsar.Client
	p  pulsar.Producer
}

func NewPulsar(url, topic string) (*Pulsar, error) {
	cl, err := pulsar.NewClient(pulsar.ClientOptions{URL: url})
	if err != nil {
		return nil, err
	}

	p, err := cl.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})

	return &Pulsar{cl: cl, p: p}, nil
}

func (p *Pulsar) Produce(ctx context.Context, topic, key, value string) error {
	_, err := p.p.Send(ctx, &pulsar.ProducerMessage{
		Payload: []byte(value),
		Key:     key,
	})

	return err
}

func (p *Pulsar) Consume(ctx context.Context, topic string) (chan Message, error) {
	ch := make(chan Message)
	var c pulsar.Consumer
	go func() {
		<-ctx.Done()
		close(ch)
		p.p.Close()
		p.cl.Close()
		c.Close()
	}()

	c, err := p.cl.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: "test",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	go func() {
		for {
			m, err := c.Receive(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					continue
				}
				panic(fmt.Sprintf("consume error: %v", err))
			}

			ts := m.EventTime()

			select {
			case <-ctx.Done():
				return
			case ch <- Message{
				Value:     string(m.Payload()),
				Key:       m.Key(),
				Timestamp: &ts,
			}:
			}

			c.Ack(m)
		}
	}()

	return ch, nil
}
