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

func runTopic(ctx context.Context, c Consumer, p Producer, msgSize int, N, M, rate int, topic string) []time.Duration {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	latencies := make([]time.Duration, N)
	start := time.Now().UnixNano()

	ch, err := c.Consume(ctx, topic)
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
			ns, err := strconv.ParseInt(msg.Value[:19], 10, 64)
			if err != nil {
				panic(err)
			}
			// skip stale messages
			if ns < start {
				continue
			}

			atomic.AddInt64(&rxN, 1)
			latencies = append(latencies, time.Since(time.Unix(0, ns)))
			i++

			// Stop if number of messages is reached.
			if N > 0 && i == N {
				return
			}

			// Stop if number of minutes is reached.
			if M > 0 && (time.Now().UnixNano()-start)/int64(time.Minute) == int64(M) {
				return
			}
		}
	}()

	// Produce.
	i := 0
	lastProduced := time.Time{}
	for {
		// Limit produce rate.
		if !lastProduced.IsZero() {
			diff := time.Until(lastProduced.Add(time.Second / time.Duration(rate)))
			if diff > 0 {
				time.Sleep(diff)
			}
		}

		var b strings.Builder
		b.Grow(msgSize)
		ts := strconv.FormatInt(time.Now().UnixNano(), 10)
		b.WriteString(ts)
		for n := 0; n < msgSize-len(ts); n++ {
			b.WriteByte(42)
		}

		if err := p.Produce(ctx, topic, "", b.String()); err != nil {
			log.Printf("failed to produce: %v", err)
			break
		}

		atomic.AddInt64(&txN, 1)
		lastProduced = time.Now()
		i++

		// Stop if number of messages is reached.
		if N > 0 && i == N {
			break
		}
		// Stop if number of minutes is reached.
		if M > 0 && (time.Now().UnixNano()-start)/int64(time.Minute) == int64(M) {
			break
		}
	}

	wg.Wait()

	return latencies
}

func RunBench(ctx context.Context, msgSize int, N, M, rate int, brokerType, brokerURLs, topics string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	latencies := make([]time.Duration, 0, N*len(strings.Split(topics, ",")))
	start := time.Now()

	wg := sync.WaitGroup{}
	ch := make(chan []time.Duration, 10)

	for _, topic := range strings.Split(topics, ",") {
		wg.Add(1)
		var (
			p Producer
			c Consumer
		)
		switch brokerType {
		case "pulsar":
			k, err := brokers.NewPulsar(brokerURLs, topic)
			if err != nil {
				log.Fatalf("failed to create Pulsar client: %v", err)
			}
			c = k
			p = k
		case "nats":
			k, err := brokers.NewNats(brokerURLs, topic)
			if err != nil {
				log.Fatalf("failed to create Nats JetStream client: %v", err)
			}
			c = k
			p = k
		case "kafka":
			k := brokers.NewKafka(brokerURLs, topic)
			c = k
			p = k
		case "redpanda":
			rp, err := brokers.NewRedPanda(brokerURLs, topic)
			if err != nil {
				log.Fatalf("failed to create RedPanda client: %v", err)
			}
			c = rp
			p = rp
		}
		go func(topic string, p Producer, c Consumer) {
			defer wg.Done()
			ch <- runTopic(ctx, c, p, msgSize, N, M, rate, topic)
		}(topic, p, c)
	}

	// Append latencies.
	wgl := sync.WaitGroup{}
	wgl.Add(1)
	go func() {
		defer wgl.Done()
		for ls := range ch {
			// Trim first 10% of latencies slice
			// to account for broker "warm up" time.
			latencies = append(latencies, ls[len(ls)/10:]...)
		}
	}()

	// Print progress.
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

	wg.Wait()
	cancel()
	close(ch)
	wgl.Wait()

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

	N = len(latencies) // This one is smaller than rxN because we trim first 10% of warmup time.
	fmt.Printf("Min latency: %v\n", ms(latencies[0]))
	fmt.Printf("P90 latency: %v\n", ms(latencies[N-N/10]))
	fmt.Printf("P99 latency: %v\n", ms(latencies[N-N/100]))
	fmt.Printf("P99.9 latency: %v\n", ms(latencies[N-N/1000]))
	fmt.Printf("Max latency: %v\n", ms(latencies[N-1]))
	fmt.Printf("Total elapsed time: %v\n", time.Since(start))
}

func ms(d time.Duration) string {
	return fmt.Sprintf("%d ms.", d.Milliseconds())
}

func main() {
	var (
		url         string
		topics      string
		msgSize     int
		numMessages int
		minutes     int
		broker      string
		rate        int
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.StringVar(&url, "brokers", "", "url or list of broker urls comma separated")
	flag.StringVar(&topics, "topics", "topic", "comma separated list of topic names")
	flag.IntVar(&msgSize, "msg_size", 128, "message size")
	flag.IntVar(&numMessages, "num_messages", 0, "number of messages to send per topic")
	flag.IntVar(&minutes, "minutes", 0, "number of minutes to run the benchmark")
	flag.StringVar(&broker, "broker", "redpanda", "broker to test (kafka, redpanda, nats, pulsar)")
	flag.IntVar(&rate, "producer_rate", 1000, "number of messages per second to produce per topic")
	flag.Parse()

	if url == "" {
		log.Fatal("Provide at least one broker url")
	}

	if (minutes != 0 && numMessages != 0) || (minutes == 0 && numMessages == 0) {
		log.Fatal("Provide either -minutes or -num_messages, but not both.")
	}

	RunBench(ctx, msgSize, numMessages, minutes, rate, broker, url, topics)
}
