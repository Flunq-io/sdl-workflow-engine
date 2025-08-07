package processor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/flunq-io/shared/pkg/cloudevents"
	"github.com/flunq-io/executor/internal/executor"
)

// MockEventStore is a mock implementation of EventStore
type MockEventStore struct {
	mock.Mock
}

func (m *MockEventStore) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	args := m.Called(ctx, workflowID)
	return args.Get(0).([]*cloudevents.CloudEvent), args.Error(1)
}

// MockTaskExecutor is a mock implementation of TaskExecutor
type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) Execute(ctx context.Context, task *executor.TaskRequest) (*executor.TaskResult, error) {
	args := m.Called(ctx, task)
	return args.Get(0).(*executor.TaskResult), args.Error(1)
}

func (m *MockTaskExecutor) GetTaskType() string {
	args := m.Called()
	return args.String(0)
}

func TestPublishTaskCompletedEvent(t *testing.T) {
	// Setup
	mockEventStore := new(MockEventStore)
	logger := zap.NewNop()

	processor := &TaskProcessor{
		eventStore: mockEventStore,
		logger:     logger,
	}

	// Create a task result with WorkflowID and ExecutionID
	taskResult := &executor.TaskResult{
		TaskID:      "test-task-123",
		TaskName:    "test-task",
		WorkflowID:  "test-workflow-456",
		ExecutionID: "test-execution-789",
		Success:     true,
		Output:      map[string]interface{}{"result": "success"},
		Duration:    100 * time.Millisecond,
		ExecutedAt:  time.Now(),
	}

	// Setup mock expectation
	mockEventStore.On("PublishEvent", mock.Anything, mock.MatchedBy(func(event *cloudevents.CloudEvent) bool {
		// Verify the event has the correct structure
		assert.Equal(t, "io.flunq.task.completed", event.Type)
		assert.Equal(t, "test-workflow-456", event.WorkflowID)
		assert.Equal(t, "test-execution-789", event.ExecutionID)
		assert.Equal(t, "task-completed-test-task-123", event.ID)
		assert.Equal(t, "executor-service", event.Source)

		// Verify the event data
		data, ok := event.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "test-task-123", data["task_id"])
		assert.Equal(t, "test-task", data["task_name"])
		assert.Equal(t, true, data["success"])
		assert.NotNil(t, data["output"])
		assert.NotNil(t, data["duration_ms"])
		assert.NotNil(t, data["executed_at"])

		return true
	})).Return(nil)

	// Execute
	err := processor.publishTaskCompletedEvent(context.Background(), taskResult, "test-tenant")

	// Assert
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
}
