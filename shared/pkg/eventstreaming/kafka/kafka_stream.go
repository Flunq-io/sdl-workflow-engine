package kafka

import (
	"context"
	"fmt"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// Stream is a placeholder Kafka EventStream implementation.
// It compiles and returns clear errors so contributors can fill it in.
type Stream struct {
	logger interfaces.Logger
}

func NewKafkaStream(logger interfaces.Logger) *Stream {
	if logger == nil {
		panic("kafka stream requires a non-nil logger")
	}
	return &Stream{logger: logger}
}

func (s *Stream) Subscribe(ctx context.Context, filters interfaces.StreamFilters) (interfaces.StreamSubscription, error) {
	return nil, fmt.Errorf("kafka event stream: Subscribe not implemented yet; contribute at shared/pkg/eventstreaming/kafka")
}

func (s *Stream) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
	return fmt.Errorf("kafka event stream: Publish not implemented yet; contribute at shared/pkg/eventstreaming/kafka")
}

func (s *Stream) CreateConsumerGroup(ctx context.Context, groupName string) error {
	return fmt.Errorf("kafka event stream: CreateConsumerGroup not implemented yet; contribute at shared/pkg/eventstreaming/kafka")
}

func (s *Stream) DeleteConsumerGroup(ctx context.Context, groupName string) error {
	return fmt.Errorf("kafka event stream: DeleteConsumerGroup not implemented yet; contribute at shared/pkg/eventstreaming/kafka")
}

func (s *Stream) GetStreamInfo(ctx context.Context) (*interfaces.StreamInfo, error) {
	return nil, fmt.Errorf("kafka event stream: GetStreamInfo not implemented yet; contribute at shared/pkg/eventstreaming/kafka")
}

func (s *Stream) Close() error {
	return nil
}

