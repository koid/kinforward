package dogstatsd

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
)

// Statsd is a statreceiver that writes stats to a statsd endpoint
type Statsd struct {
	client *statsd.Client
}

// New creates a new Statsd statreceiver with a new instance of a cactus statter
func New(addr string, tags []string) (*Statsd, error) {
	sd, err := statsd.NewBuffered(addr, 10000)
	if err != nil {
		return nil, err
	}
	sd.SkipErrors = true
	sd.Tags = tags
	return &Statsd{
		client: sd,
	}, nil
}

// Checkpoint implementation that writes to statsd
func (s *Statsd) Checkpoint() {
	_ = s.client.Incr("kinsumer.checkpoints", nil, 1.0)
}

// EventToClient implementation that writes to statsd metrics about a record
// that was consumed by the client
func (s *Statsd) EventToClient(inserted, retrieved time.Time) {
	now := time.Now()
	_ = s.client.Incr("kinsumer.consumed", nil, 1.0)
	_ = s.client.Timing("kinsumer.in_stream", retrieved.Sub(inserted), nil, 1.0)
	_ = s.client.Timing("kinsumer.end_to_end", now.Sub(inserted), nil, 1.0)
	_ = s.client.Timing("kinsumer.in_kinsumer", now.Sub(retrieved), nil, 1.0)
}

// EventsFromKinesis implementation that writes to statsd metrics about records that
// were retrieved from kinesis
func (s *Statsd) EventsFromKinesis(num int, shardID string, lag time.Duration) {
	tags := []string{
		fmt.Sprintf("shardId:%s", shardID),
	}
	_ = s.client.Timing("kinsumer.lag", lag, tags, 1.0)
	_ = s.client.Count("kinsumer.retrieved", int64(num), tags, 1.0)
}
