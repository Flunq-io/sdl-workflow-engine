package factory

import (
	"fmt"

	redis "github.com/go-redis/redis/v8"

	"github.com/flunq-io/shared/pkg/eventstreaming"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// EventStreamDeps encapsulates dependencies to construct an EventStream implementation.
// Extend via Options in interfaces.StreamConfig for additional backends.
type EventStreamDeps struct {
	Backend     string                // e.g. "redis", "kafka"
	RedisClient *redis.Client         // used when Backend == "redis"
	Logger      eventstreaming.Logger // logger interface expected by eventstream implementations
}

// NewEventStream returns a shared EventStream implementation based on deps.Backend.
// Unsupported backends return a clear error inviting contributions.
func NewEventStream(deps EventStreamDeps) (interfaces.EventStream, error) {
	switch deps.Backend {
	case "", "redis":
		if deps.RedisClient == nil {
			return nil, fmt.Errorf("redis backend selected but RedisClient is nil")
		}
		return eventstreaming.NewRedisEventStream(deps.RedisClient, deps.Logger), nil
	case "kafka":
		return nil, fmt.Errorf("kafka backend not implemented yet; contribute an implementation in shared/pkg/eventstreaming/kafka")
	case "rabbitmq":
		return nil, fmt.Errorf("rabbitmq backend not implemented yet; contribute an implementation in shared/pkg/eventstreaming/rabbitmq")
	case "nats":
		return nil, fmt.Errorf("nats backend not implemented yet; contribute an implementation in shared/pkg/eventstreaming/nats")
	default:
		return nil, fmt.Errorf("unsupported EVENT_STREAM_TYPE backend: %s", deps.Backend)
	}
}
