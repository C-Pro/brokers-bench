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

Obviously Kafka struggled under load, while Redpanda performed quite good. I believe Kafka performance might be improved by tuning JVM garbage collector, but I do not have a desire to research on how to do it. To be fair I did not tune any of the brokers in the test at all.

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
