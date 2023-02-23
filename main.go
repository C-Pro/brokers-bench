package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	var (
		url         string
		topics      string
		msgSize     int
		numMessages int
		minutes     int
		broker      string
		rate        int
		producers   int
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.StringVar(&url, "brokers", "", "url or list of broker urls comma separated")
	flag.StringVar(&topics, "topics", "topic", "comma separated list of topic names")
	flag.IntVar(&msgSize, "msg_size", 128, "message size")
	flag.IntVar(&numMessages, "num_messages", 0, "number of messages to send per producer (by default there's one producer per topic)")
	flag.IntVar(&minutes, "minutes", 0, "number of minutes to run the benchmark")
	flag.StringVar(&broker, "driver", "redpanda", "driver to use (kafka, redpanda, nats, pulsar)")
	flag.IntVar(&rate, "producer_rate", 1000, "number of messages per second to produce per producer (by default there's one producer per topic)")
	flag.IntVar(&producers, "producers_per_topic", 1, "number producers per topic")
	flag.Parse()

	if url == "" {
		log.Fatal("Provide at least one broker url")
	}

	if (minutes != 0 && numMessages != 0) || (minutes == 0 && numMessages == 0) {
		log.Fatal("Provide either -minutes or -num_messages, but not both.")
	}

	RunBench(ctx, msgSize, numMessages, minutes, rate, producers, broker, url, topics)
}
