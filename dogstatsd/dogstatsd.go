package dogstatsd

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
)

// Statsd is a statreceiver that writes stats to a statsd endpoint
type Statsd struct {
	client *statsd.Client
	tags   []string
}

// New creates a new Statsd statreceiver with a new instance of a cactus statter
func New(addr string, tags []string) (*Statsd, error) {
	sd, err := statsd.New(addr)
	if err != nil {
		return nil, err
	}
	return &Statsd{
		client: sd,
		tags:   tags,
	}, nil
}

// Checkpoint implementation that writes to statsd
func (s *Statsd) Checkpoint() {
	if s.client == nil {
		return
	}

	_ = s.client.Incr("kinsumer.checkpoints", s.tags, 1.0)
}

// EventToClient implementation that writes to statsd metrics about a record
// that was consumed by the client
func (s *Statsd) EventToClient(inserted, retrieved time.Time) {
	if s.client == nil {
		return
	}

	now := time.Now()
	_ = s.client.Incr("kinsumer.consumed", s.tags, 1.0)
	_ = s.client.Timing("kinsumer.in_stream", retrieved.Sub(inserted), s.tags, 1.0)
	_ = s.client.Timing("kinsumer.end_to_end", now.Sub(inserted), s.tags, 1.0)
	_ = s.client.Timing("kinsumer.in_kinsumer", now.Sub(retrieved), s.tags, 1.0)
}

// EventsFromKinesis implementation that writes to statsd metrics about records that
// were retrieved from kinesis
func (s *Statsd) EventsFromKinesis(num int, shardID string, lag time.Duration) {
	if s.client == nil {
		return
	}

	shardTag := fmt.Sprintf("shardId:%s", shardID)
	_ = s.client.Timing("kinsumer.lag", lag, append(s.tags, shardTag), 1.0)
	_ = s.client.Count("kinsumer.retrieved", int64(num), append(s.tags, shardTag), 1.0)
}
