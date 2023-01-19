package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"streambench/brokers"
)

var (
	txN int64
	rxN int64
)

type Producer interface {
	Produce(ctx context.Context, topic, key, value string) error
}

type Consumer interface {
	Consume(ctx context.Context, topic string) (chan brokers.Message, error)
}

func RunBench(ctx context.Context, c Consumer, p Producer, msgSize int, N int) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	latencies := make([]time.Duration, N)
	start := time.Now()

	ch, err := c.Consume(ctx, "topic")
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	// Print progress
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			rx := atomic.LoadInt64(&rxN)
			tx := atomic.LoadInt64(&txN)

			mps := 0
			mbps := 0.0
			elapsed := time.Since(start)
			if rx > 0 && elapsed.Seconds() >= 1 {
				mps = int(float64(rx) / elapsed.Seconds())
				mbps = float64(rx*int64(msgSize)) / elapsed.Seconds() / 1024 / 1024
			}

			log.Printf("Produced: %d, Consumed: %d (%d messages/sec, %.2f Mb/sec)", tx, rx, mps, mbps)
		}
	}()

	// Consume
	go func() {
		defer wg.Done()
		start := time.Now().UnixNano()
		for msg := range ch {
			ns, err := strconv.ParseInt(msg.Value[:19], 10, 64)
			if err != nil {
				panic(err)
			}
			// skip stale messages
			if ns < start {
				continue
			}
			i := atomic.AddInt64(&rxN, 1)
			latencies[int(i-1)] = time.Since(time.Unix(0, ns))

			if int(i) == N-1 {
				return
			}
		}
	}()

	// Produce
	for i := 0; i < N; i++ {
		var b strings.Builder
		b.Grow(msgSize)
		ts := strconv.FormatInt(time.Now().UnixNano(), 10)
		b.WriteString(ts)
		for n := 0; n < msgSize-len(ts); n++ {
			b.WriteByte(42)
		}

		if err := p.Produce(ctx, "topic", "", b.String()); err != nil {
			log.Printf("failed to produce: %v", err)
			break
		}

		atomic.AddInt64(&txN, 1)
	}

	wg.Wait()
	cancel()

	elapsed := time.Since(start)
	N = int(atomic.LoadInt64(&rxN))
	if N == 0 {
		log.Printf("No messages received in %v", elapsed)
		return
	}

	fmt.Printf("Message throughput: %.2f messages/sec\n", float64(N)/elapsed.Seconds())
	fmt.Printf("Data throughput: %f Mb/sec\n", (float64(N*msgSize)/elapsed.Seconds())/1024/1024)

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	fmt.Printf("Min latency: %v\n", latencies[0])
	fmt.Printf("P90 latency: %v\n", latencies[N-N/10])
	fmt.Printf("P99 latency: %v\n", latencies[N-N/100])
	fmt.Printf("Max latency: %v\n", latencies[N-1])
}

func main() {
	var (
		p Producer
		c Consumer

		url         string
		topic       string
		msgSize     int
		numMessages int
		broker      string
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.StringVar(&url, "brokers", "", "url or list of broker urls comma separated")
	flag.StringVar(&topic, "topic", "topic", "topic name")
	flag.IntVar(&msgSize, "msg_size", 128, "message size")
	flag.IntVar(&numMessages, "num_messages", 10000, "number of messages to send")
	flag.StringVar(&broker, "broker", "redpanda", "broker to test (kafka, redpanda, nats, pulsar)")
	flag.Parse()

	if url == "" {
		log.Fatal("Provide at least one broker url")
	}

	switch broker {
	case "pulsar":
		k, err := brokers.NewPulsar(url, topic)
		if err != nil {
			log.Fatalf("failed to create Pulsar client: %v", err)
		}
		c = k
		p = k
	case "nats":
		k, err := brokers.NewNats(url, topic)
		if err != nil {
			log.Fatalf("failed to create Nats JetStream client: %v", err)
		}
		c = k
		p = k
	case "kafka":
		k := brokers.NewKafka(url, topic)
		c = k
		p = k
	case "redpanda":
		rp, err := brokers.NewRedPanda(url, topic)
		if err != nil {
			log.Fatalf("failed to create RedPanda client: %v", err)
		}
		c = rp
		p = rp
	}

	RunBench(ctx, c, p, msgSize, numMessages)
}
