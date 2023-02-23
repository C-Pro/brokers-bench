# Message queue benchmark

The purpose of this benchmark was to compare several message brokers in a very specific use case:
Robust guaranteed delivery of a strictly ordered stream of events.

So during the test all brokers were configured in 3 node synchronous replication. Messages were sent without batching, in a single stream to a single partition with mandatory replication to all three nodes. The main parameter that I looked at was latency.

I have tested four brokers / messaging systems:
* Apache Kafka
* Redpanda
* NATS JetStream
* Apache Pulsar

That's what I got:

## Kafka
```
Message throughput: 594.53 messages/sec
Data throughput: 0.072575 Mb/sec
Min latency: 0s
P90 latency: 1.838ms
P99 latency: 3.06ms
Max latency: 21.929ms
```

## Redpanda
```
Message throughput: 1395.50 messages/sec
Data throughput: 0.170350 Mb/sec
Min latency: 0s
P90 latency: 10.013ms
P99 latency: 10.985ms
Max latency: 16.786ms
```

## Nats JetStream (with ACKs)
```
Message throughput: 1230.48 messages/sec
Data throughput: 0.150206 Mb/sec
Min latency: 0s
P90 latency: 1.019ms
P99 latency: 3.53ms
Max latency: 8.987ms
```

## Apache Pulsar

I had hard time configuring and running Apache Pulsar and results were quite bad performance-wize, so it can be disregarded here.

```
Message throughput: 92.56 messages/sec
Data throughput: 0.011299 Mb/sec
Min latency: 0s
P90 latency: 19.376075ms
P99 latency: 32.94208ms
Max latency: 513.493147ms
```

For Kafka, Nats, Pulsar I used compose files provided in the repository. For Redpanda I used `rpk` tool from homebrew to create the cluster and a topic.

All tests except for Pulsar were done on MBP m2 32gb. Pulsar docker images keeped crashing on mac, so I tested it on AWS EC2 i3.large instance.
I don't think it is a reason for its poor performance, more likely the configuration (the only running config I found has two clusters, and can't be stripped to one by just removing entries in compose) is just not optimized for the usecase. But having spent 3x more time on Pulsar then on other 3 brokers combined I just gave up because I would not recommend it to anyone I don't hate anyway :)


## Ten topics

I also ran a benchmark with 10 topics in parallel for Kafka and Redpanda (as we narrowed down our selection to this two options by this point).
Each topic had one partition and replication factor 3.
Producers were set up to wait acknlowledge from all of the replicas before publishing next message.
There were 10 producers and 10 consumers. Each producer sent 100_000 messages, so total number of sent/received messages in the test is one million.

Redpanda:
```
Message throughput: 2949.83 messages/sec
Data throughput: 2.880691 Mb/sec
Min latency: 613µs
P90 latency: 12.929ms
P99 latency: 17.362ms
P99.9 latency: 29.048ms
```

Kafka:
```
Message throughput: 1477.08 messages/sec
Data throughput: 1.442457 Mb/sec
Min latency: 1.583ms
P90 latency: 11.287ms
P99 latency: 27.341ms
P99.9 latency: 49.532ms
```

The throughput shows the brokers are still far from saturation, so I disabled waiting for acks at all to speed up producers and saturate the throughput.

Kafka:
```
Message throughput: 7463.01 messages/sec
Data throughput: 7.288099 Mb/sec
Min latency: 1.612ms
P90 latency: 3.10843s
P99 latency: 6.866796s
P99.9 latency: 8.339648s
```

Redpanda:
```
Message throughput: 36776.07 messages/sec
Data throughput: 35.914130 Mb/sec
Min latency: 18.364ms
P90 latency: 7.302656s
P99 latency: 10.98988s
P99.9 latency: 11.505453s
```

Obviously both Kafka and Redpanda struggled under load, but Redpanda througput was about 5 times better.

And the last benchmark with waiting for only one acknowledge (from leader broker for the respective partition).

Kafka:
```
Message throughput: 2856.32 messages/sec
Data throughput: 2.789373 Mb/sec
Min latency: 1.649ms
P90 latency: 18.452ms
P99 latency: 52.439ms
P99.9 latency: 115.218ms
```

Redpanda:
```
Message throughput: 5434.38 messages/sec
Data throughput: 5.307011 Mb/sec
Min latency: 409µs
P90 latency: 13.113ms
P99 latency: 18.431ms
P99.9 latency: 24.214ms
```

## Compression effect

Several benchmarks to measure message compression effect on message throughput and latency.
Setup is:
* Three brokers running in Docker on MacOS m2 CPU (VM with 2 CPU cores and 24Gb RAM).
* Ten topics with one partition each.
* Replication factor is 3.
* One producer and one consumer per topic.
* Redpanda (franz-go) driver.
* Producers are waiting for ack from a partiton leader (for Redpanda it means fsync of message on the leader).

### 1kb message size.

No compression. Leader ack. Redpanda.

```
Data throughput: 4.289471 Mb/sec
Min latency: 0 ms.
P90 latency: 12 ms.
P99 latency: 16 ms.
P99.9 latency: 20 ms.
Max latency: 32 ms.
Total elapsed time: 1m0.045717625s
```

Gzip compression. Leader ack. Redpanda.

```
Message throughput: 4408.45 messages/sec
Data throughput: 4.305127 Mb/sec
Min latency: 0 ms.
P90 latency: 12 ms.
P99 latency: 16 ms.
P99.9 latency: 19 ms.
Max latency: 27 ms.
Total elapsed time: 1m0.061644166s
```

Snappy compression. Leader ack. Redpanda.

```
Message throughput: 4482.91 messages/sec
Data throughput: 4.377841 Mb/sec
Min latency: 0 ms.
P90 latency: 12 ms.
P99 latency: 16 ms.
P99.9 latency: 20 ms.
Max latency: 30 ms.
Total elapsed time: 1m0.048094875s
```

## 10 Kb message size

No compression. Leader ack. Redpanda.

```
Message throughput: 4122.12 messages/sec
Data throughput: 40.255121 Mb/sec
Min latency: 0 ms.
P90 latency: 11 ms.
P99 latency: 15 ms.
P99.9 latency: 19 ms.
Max latency: 30 ms.
Total elapsed time: 1m0.041290917s
```

Gzip compression. Leader ack. Redpanda.

```
Message throughput: 4196.52 messages/sec
Data throughput: 40.981668 Mb/sec
Min latency: 0 ms.
P90 latency: 11 ms.
P99 latency: 16 ms.
P99.9 latency: 20 ms.
Max latency: 47 ms.
Total elapsed time: 1m0.050047s
```

Snappy compression. Leader ack. Redpanda.

```
Message throughput: 4372.22 messages/sec
Data throughput: 42.697422 Mb/sec
Min latency: 0 ms.
P90 latency: 12 ms.
P99 latency: 15 ms.
P99.9 latency: 19 ms.
Max latency: 30 ms.
Total elapsed time: 1m0.038668375s
```

## Multiple producers

Benchmarking several producers writing into one topic.
Setup is:
* Three brokers running in Docker on MacOS m2 CPU (VM with 2 CPU cores and 24Gb RAM).
* Ten topics with one partition each (except in last test, where I partitioned all topics).
* Replication factor is 3.
* One producer and one consumer per topic.
* Redpanda (franz-go) driver.
* Producers are waiting for ack from a partiton leader (for Redpanda it means fsync of message on the leader).

For for Kafka benchmark is running for 5 minutes so JVM will have enough time to "warm up".
Redpanda benchmarks are running for 1 minute, because there's no significant deviation in performance during the run.
For all benchmarks first 10% of latency readings are discarded before computing percentiles.

Single topic. Redpanda.

```
Message throughput: 5343.15 messages/sec
Data throughput: 5.217921 Mb/sec
Min latency: 0 ms.
P90 latency: 10 ms.
P99 latency: 13 ms.
P99.9 latency: 16 ms.
Max latency: 22 ms.
Total elapsed time: 1m0.0275945s
```

Single topic. Kafka.

```
Message throughput: 4372.17 messages/sec
Data throughput: 4.269692 Mb/sec
Min latency: 0 ms.
P90 latency: 6 ms.
P99 latency: 11 ms.
P99.9 latency: 20 ms.
Max latency: 38 ms.
Total elapsed time: 5m0.097158125s
Commandline arguments: -driver redpanda -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9095 -topics=topic_1 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Four topics (40 producers and 4 consumers). Redpanda with franz-go driver.

```
Message throughput: 9933.75 messages/sec
Data throughput: 9.700924 Mb/sec
Min latency: 0 ms.
P90 latency: 15 ms.
P99 latency: 23 ms.
P99.9 latency: 29 ms.
Max latency: 46 ms.
Total elapsed time: 1m0.073410667s
```

Four topics (40 producers and 4 consumers). Redpanda, with kafka-go driver.
```
Message throughput: 7687.55 messages/sec
Data throughput: 7.507369 Mb/sec
Min latency: 0 ms.
P90 latency: 18 ms.
P99 latency: 26 ms.
P99.9 latency: 34 ms.
Max latency: 63 ms.
Total elapsed time: 5m0.188373708s
Commandline arguments: -driver kafka -brokers=127.0.0.1:63574,127.0.0.1:63573,127.0.0.1:63570 -topics=topic_1,topic_2,topic_3,topic_4 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Four topics (40 producers and 4 consumers). Kafka with franz-go driver.

```
Message throughput: 7147.80 messages/sec
Data throughput: 6.980273 Mb/sec
Min latency: 0 ms.
P90 latency: 56 ms.
P99 latency: 108 ms.
P99.9 latency: 160 ms.
Max latency: 241 ms.
Total elapsed time: 5m0.25113875s
Commandline arguments: -driver redpanda -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9095 -topics=topic_1,topic_2,topic_3,topic_4 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Four topics (40 producers and 4 consumers). Kafka with kafka-go driver.

```
Message throughput: 6496.36 messages/sec
Data throughput: 6.344100 Mb/sec
Min latency: 1 ms.
P90 latency: 18 ms.
P99 latency: 28 ms.
P99.9 latency: 42 ms.
Max latency: 127 ms.
Total elapsed time: 5m0.183461583s
Commandline arguments: -driver kafka -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9094 -topics=topic_1,topic_2,topic_3,topic_4 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Ten topics (100 producers and 10 consumers). Redpanda.

```
Message throughput: 11525.34 messages/sec
Data throughput: 11.255215 Mb/sec
Min latency: 0 ms.
P90 latency: 37 ms.
P99 latency: 55 ms.
P99.9 latency: 71 ms.
Max latency: 101 ms.
Total elapsed time: 1m0.089733583s
```

Ten topics (100 producers and 10 consumers). Kafka.

```
Message throughput: 8487.30 messages/sec
Data throughput: 8.288379 Mb/sec
Min latency: 1 ms.
P90 latency: 120 ms.
P99 latency: 216 ms.
P99.9 latency: 319 ms.
Max latency: 506 ms.
Total elapsed time: 5m0.365870125s
Commandline arguments: -driver redpanda -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9095 -topics=topic_1,topic_2,topic_3,topic_4,topic_5,topic_6,topic_7,topic_8,topic_9,topic_10 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Ten topics with 10 partitions each (100 producers and 10 consumers). Redpanda.

```
Message throughput: 11882.75 messages/sec
Data throughput: 11.604249 Mb/sec
Min latency: 1 ms.
P90 latency: 73 ms.
P99 latency: 116 ms.
P99.9 latency: 141 ms.
Max latency: 179 ms.
Total elapsed time: 1m0.098068875s
```

Ten topics with 10 partitions each (100 producers and 10 consumers). Kafka.

```
Message throughput: 7161.17 messages/sec
Data throughput: 6.993326 Mb/sec
Min latency: 2 ms.
P90 latency: 145 ms.
P99 latency: 248 ms.
P99.9 latency: 343 ms.
Max latency: 742 ms.
Total elapsed time: 5m0.349696209s
Commandline arguments: -driver redpanda -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9095 -topics=topic_1,topic_2,topic_3,topic_4,topic_5,topic_6,topic_7,topic_8,topic_9,topic_10 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Just for fun there's a NATS benchmark with the same scenario: 1 stream with 3 replicas on 3 brokers, 10 subjects, 100 producers, 10 consumers.

```
Message throughput: 7971.84 messages/sec
Data throughput: 7.784997 Mb/sec
Min latency: 0 ms.
P90 latency: 27 ms.
P99 latency: 61 ms.
P99.9 latency: 89 ms.
Max latency: 124 ms.
Total elapsed time: 1m0.065176292s
Commandline arguments: -driver nats -brokers 127.0.0.1:4222,127.0.0.1:4223,127.0.0.1:4224 -topics=s0,s1,s2,s3,s4,s5,s6,s7,s8,s9 -msg_size=1024 -minutes=1 -producers_per_topic=10
```

## Vertical scaling

In the next test I added 8 CPU cores to the previous setup (total 10 cores). Everything else is unchanged.

Kafka

```
Message throughput: 21539.27 messages/sec
Data throughput: 21.034442 Mb/sec
Min latency: 1 ms.
P90 latency: 11 ms.
P99 latency: 20 ms.
P99.9 latency: 36 ms.
Max latency: 127 ms.
Total elapsed time: 5m0.627718333s
Commandline arguments: -driver redpanda -brokers=192.168.3.148:9093,192.168.3.148:9094,192.168.3.148:9095 -topics=topic_1,topic_2,topic_3,topic_4,topic_5,topic_6,topic_7,topic_8,topic_9,topic_10 -msg_size=1024 -minutes=5 -producers_per_topic=10
```

Redpanda

```
Message throughput: 20974.61 messages/sec
Data throughput: 20.483016 Mb/sec
Min latency: 0 ms.
P90 latency: 13 ms.
P99 latency: 19 ms.
P99.9 latency: 35 ms.
Max latency: 60 ms.
Total elapsed time: 1m0.1442035s
Commandline arguments: -driver redpanda -brokers=127.0.0.1:53022,127.0.0.1:53027,127.0.0.1:53026 -topics=topic_1,topic_2,topic_3,topic_4,topic_5,topic_6,topic_7,topic_8,topic_9,topic_10 -msg_size=1024 -minutes=1 -producers_per_topic=10
```


Nats

```
Message throughput: 16148.69 messages/sec
Data throughput: 15.770207 Mb/sec
Min latency: 0 ms.
P90 latency: 8 ms.
P99 latency: 15 ms.
P99.9 latency: 28 ms.
Max latency: 58 ms.
Total elapsed time: 1m0.08941725s
Commandline arguments: -driver nats -brokers 127.0.0.1:4222,127.0.0.1:4223,127.0.0.1:4224 -topics=s0,s1,s2,s3,s4,s5,s6,s7,s8,s9 -msg_size=1024 -minutes=1 -producers_per_topic=10
```
