package processor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
)

// captureEventStream is a minimal EventStream test double that records published events
// and stubs other methods.
type captureEventStream struct{ Published []*cloudevents.CloudEvent }

func (c *captureEventStream) Subscribe(ctx context.Context, filters sharedinterfaces.StreamFilters) (sharedinterfaces.StreamSubscription, error) {
	return nil, nil
}
func (c *captureEventStream) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
	c.Published = append(c.Published, event)
	return nil
}
func (c *captureEventStream) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	return nil, nil
}
func (c *captureEventStream) CreateConsumerGroup(ctx context.Context, groupName string) error { return nil }
func (c *captureEventStream) DeleteConsumerGroup(ctx context.Context, groupName string) error { return nil }
func (c *captureEventStream) GetStreamInfo(ctx context.Context) (*sharedinterfaces.StreamInfo, error) {
	return &sharedinterfaces.StreamInfo{}, nil
}
func (c *captureEventStream) Close() error { return nil }

func TestTimerWaitScheduler_PublishesScheduledEvent(t *testing.T) {
	es := &captureEventStream{}
	s := NewTimerWaitScheduler(es)

	ctx := context.Background()
	tenant := "acme-inc"
	wf := "wf-123"
	exec := "exec-456"
	task := "wait-task-1"
	dur := 2 * time.Second

	err := s.Schedule(ctx, tenant, wf, exec, task, dur)
	assert.NoError(t, err)
	if assert.Len(t, es.Published, 1) {
		ev := es.Published[0]
		assert.Equal(t, "io.flunq.timer.scheduled", ev.Type)
		assert.Equal(t, tenant, ev.TenantID)
		assert.Equal(t, wf, ev.WorkflowID)
		assert.Equal(t, exec, ev.ExecutionID)
		assert.Equal(t, task, ev.TaskID)

		// Verify core payload fields
		m, _ := ev.Data.(map[string]interface{})
		assert.Equal(t, tenant, m["tenant_id"])
		assert.Equal(t, wf, m["workflow_id"])
		assert.Equal(t, exec, m["execution_id"])
		assert.Equal(t, task, m["task_id"])
		assert.Contains(t, m, "fire_at")
	}
}

