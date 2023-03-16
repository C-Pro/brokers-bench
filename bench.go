package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"streambench/brokers"

	"gonum.org/v1/gonum/stat"
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

func runTopic(ctx context.Context, msgSize int, N, M, rate, producers int, brokerType, brokerURLs, topic string) []time.Duration {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	latencies := make([]time.Duration, 0, N)
	start := time.Now().UnixNano()

	c := NewClient(brokerType, brokerURLs, topic)
	ch, err := c.Consume(ctx, topic)
	if err != nil {
		panic(err)
	}

	cwg := sync.WaitGroup{}
	cwg.Add(1)

	// Consume
	go func() {
		defer cwg.Done()
		i := 0
		for msg := range ch {
			ns, err := strconv.ParseInt(msg.Value[:19], 10, 64)
			if err != nil {
				panic(err)
			}
			// skip stale or future messages
			if ns < start || ns >= time.Now().UnixNano() {
				continue
			}

			atomic.AddInt64(&rxN, 1)
			latency := time.Since(time.Unix(0, ns))
			latencies = append(latencies, latency)
			i++
		}
	}()

	// Produce.
	pwg := sync.WaitGroup{}
	for pidx := 0; pidx <= producers; pidx++ {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			p := NewClient(brokerType, brokerURLs, "")
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
				ts := time.Now()
				tsStr := strconv.FormatInt(ts.UnixNano(), 10)
				b.WriteString(tsStr)
				for n := 0; n < msgSize-len(tsStr); n++ {
					b.WriteByte(42)
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := p.Produce(ctx, topic, "", b.String()); err != nil {
					log.Printf("failed to produce: %v", err)
					break
				}

				atomic.AddInt64(&txN, 1)
				lastProduced = ts
				i++

				// Stop if number of messages is reached.
				if N > 0 && i >= N {
					log.Printf("Producer %s stopping due to message count limit", topic)
					break
				}
				// Stop if number of minutes is reached.
				if M > 0 && (time.Now().UnixNano()-start)/int64(time.Minute) >= int64(M) {
					log.Printf("Producer %s stopping due to time limit", topic)
					break
				}
			}
		}()
	}

	pwg.Wait() // Wait for producers to finish.
	cancel()
	// cwg.Wait() // Wait for consumer to finish.

	return latencies
}

type Client interface {
	Producer
	Consumer
}

// NewClient returns Producer it topic is empty, and Consumer otherwize.
func NewClient(brokerType, brokerURLs, topic string) Client {
	switch brokerType {
	case "pulsar":
		k, err := brokers.NewPulsar(brokerURLs, topic)
		if err != nil {
			log.Fatalf("failed to create Pulsar client: %v", err)
		}
		return k
	case "nats":
		n, err := brokers.NewNats(brokerURLs, "s") // hardcoded stream name
		if err != nil {
			log.Fatalf("failed to create Nats JetStream client: %v", err)
		}
		return n
	case "kafka":
		k := brokers.NewKafka(brokerURLs, topic)
		return k
	case "redpanda":
		rp, err := brokers.NewRedPanda(brokerURLs, topic)
		if err != nil {
			log.Fatalf("failed to create RedPanda client: %v", err)
		}
		return rp
	}

	log.Fatalf("unknown broker type: %s", brokerType)
	return nil
}

func RunBench(ctx context.Context, msgSize int, N, M, rate, producers int, brokerType, brokerURLs, topics string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	latencies := make([]time.Duration, 0, N*len(strings.Split(topics, ",")))
	start := time.Now()

	wg := sync.WaitGroup{}
	ch := make(chan []time.Duration, 10)

	for _, topic := range strings.Split(topics, ",") {
		wg.Add(1)
		go func(topic string) {
			defer wg.Done()
			ch <- runTopic(ctx, msgSize, N, M, rate, producers, brokerType, brokerURLs, topic)
		}(topic)
	}

	// Append latencies.
	wgl := sync.WaitGroup{}
	wgl.Add(1)
	go func() {
		defer wgl.Done()
		for ls := range ch {
			// Trim first 10%  and last 10% of latencies slice
			// to account for broker "warm up" time and shutdown part (some producers can
			// finish earlier than others that will make tail of the latencies more sparse).
			cut := len(ls) / 10
			latencies = append(latencies, ls[cut:len(ls)-cut]...)
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

			log.Printf("Produced: %d, Consumed: %d (%d messages/sec, %.2f Mb/sec, running for %v)", tx, rx, mps, mbps, elapsed)
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

	flats := make([]float64, len(latencies))
	for i, l := range latencies {
		flats[i] = float64(l) / float64(time.Millisecond)
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	N = len(latencies) // This one is smaller than rxN because we trim first 10% of warmup time.
	fmt.Printf("Min latency: %v\n", ms(latencies[0]))
	fmt.Printf("P90 latency: %v\n", ms(latencies[N-N/10]))
	fmt.Printf("P99 latency: %v\n", ms(latencies[N-N/100]))
	fmt.Printf("P99.9 latency: %v\n", ms(latencies[N-N/1000]))
	fmt.Printf("Max latency: %v\n", ms(latencies[N-1]))

	stddev := stat.StdDev(flats, nil)
	fmt.Printf("Latency StdDev: %.6f\n", stddev)
	fmt.Printf("Latency StdErr: %.6f\n", stat.StdErr(stddev, float64(N)))
	fmt.Printf("Total elapsed time: %v\n", time.Since(start))
	fmt.Printf("Commandline arguments: %s\n", strings.Join(os.Args[1:], " "))

	f, err := os.OpenFile("latencies.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	defer f.Close()
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	w := csv.NewWriter(f)
	defer w.Flush()
	for i, f := range flats {
		// Sample 10% of latencies (to moving gigabytes of floats over network).
		if i%10 > 0 {
			continue
		}
		if err := w.Write([]string{strconv.FormatFloat(f, 'g', 5, 64)}); err != nil {
			log.Fatalf("failed to write csv: %v", err)
		}
	}
}

func ms(d time.Duration) string {
	return fmt.Sprintf("%d ms.", d.Milliseconds())
}
