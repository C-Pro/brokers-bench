package brokers

import "time"

type Message struct {
	Key       string
	Value     string
	Timestamp *time.Time
}
