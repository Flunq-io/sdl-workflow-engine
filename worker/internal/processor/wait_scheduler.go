package processor

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
)

// WaitScheduler schedules wait tasks via an external timer service to avoid blocking workers.
type WaitScheduler interface {
	Schedule(ctx context.Context, tenantID, workflowID, executionID, taskID string, duration time.Duration) error
}

// TimerWaitScheduler implements WaitScheduler by publishing io.flunq.timer.scheduled onto the shared EventStream.
type TimerWaitScheduler struct {
	es sharedinterfaces.EventStream
}

func NewTimerWaitScheduler(es sharedinterfaces.EventStream) *TimerWaitScheduler {
	return &TimerWaitScheduler{es: es}
}

func (t *TimerWaitScheduler) Schedule(ctx context.Context, tenantID, workflowID, executionID, taskID string, duration time.Duration) error {
	resumeAt := time.Now().Add(duration)
	ev := &cloudevents.CloudEvent{
		ID:          uuid.New().String(),
		Source:      "worker-service",
		SpecVersion: "1.0",
		Type:        "io.flunq.timer.scheduled",
		TenantID:    tenantID,
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		TaskID:      taskID,
		Time:        time.Now(),
		Data: map[string]interface{}{
			"tenant_id":    tenantID,
			"workflow_id":  workflowID,
			"execution_id": executionID,
			"task_id":      taskID,
			"task_name":    taskID,
			"duration_ms":  duration.Milliseconds(),
			"fire_at":      resumeAt.Format(time.RFC3339),
		},
	}
	return t.es.Publish(ctx, ev)
}

