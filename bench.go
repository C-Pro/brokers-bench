package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"streambench/brokers"
)

type Producer interface {
	Produce(ctx context.Context, topic, key, value string) error
}

type Consumer interface {
	Consume(ctx context.Context, topic string) (chan brokers.Message, error)
}

func RunBench(ctx context.Context, c Consumer, p Producer, msgSize int, N int) {
	if N == 0 {
		N = 1_000_000
	}
	latencies := make([]time.Duration, N)
	start := time.Now()

	ch, err := c.Consume(ctx, "topic")
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	// Consume
	go func() {
		defer wg.Done()
		i := 0
		for msg := range ch {
			ns, err := strconv.ParseInt(msg.Value[:20], 10, 64)
			if err != nil {
				panic(err)
			}
			latencies[i] = time.Since(time.Unix(0, ns))
			i++
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

		if err := p.Produce(ctx, "topic", "key-"+strconv.Itoa(i), b.String()); err != nil {
			log.Printf("failed to produce: %v", err)
		}
	}

	close(ch)
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("Message throughput: %d messages/sec", N/int(elapsed.Seconds()))
	fmt.Printf("Data throughput: %f Mb/sec", (float64(N*msgSize)/elapsed.Seconds())/1024/1024)

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	fmt.Printf("Min latency: %v", latencies[0])
	fmt.Printf("P90 latency: %v", latencies[N-N/10])
	fmt.Printf("P99 latency: %v", latencies[N-N/100])
	fmt.Printf("Max latency: %v", latencies[N-1])
}

func main() {
	k := brokers.NewKafka("localhost:9091", "topic")
	RunBench(context.Background(), k, k, 1024, 5)
}
