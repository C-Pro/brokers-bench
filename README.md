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
