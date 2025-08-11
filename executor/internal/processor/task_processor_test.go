package processor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/flunq-io/executor/internal/executor"
	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
)

// MockEventStream is a mock for the new EventStream interface used by TaskProcessor
type MockEventStream struct {
	mock.Mock
}

func (m *MockEventStream) Subscribe(ctx context.Context, filters sharedinterfaces.StreamFilters) (sharedinterfaces.StreamSubscription, error) {
	args := m.Called(ctx, filters)
	return nil, args.Error(1)
}

func (m *MockEventStream) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventStream) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	return nil, nil
}

func (m *MockEventStream) CreateConsumerGroup(ctx context.Context, groupName string) error {
	return nil
}
func (m *MockEventStream) DeleteConsumerGroup(ctx context.Context, groupName string) error {
	return nil
}
func (m *MockEventStream) GetStreamInfo(ctx context.Context) (*sharedinterfaces.StreamInfo, error) {
	return &sharedinterfaces.StreamInfo{}, nil
}
func (m *MockEventStream) ReclaimPending(ctx context.Context, consumerGroup, consumerName string, minIdleTime time.Duration) (int, error) {
	args := m.Called(ctx, consumerGroup, consumerName, minIdleTime)
	return args.Int(0), args.Error(1)
}
func (m *MockEventStream) Close() error { return nil }

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
	mockStream := new(MockEventStream)
	logger := zap.NewNop()

	processor := &TaskProcessor{
		eventStream:   mockStream,
		taskExecutors: map[string]executor.TaskExecutor{},
		logger:        logger,
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

	// Setup mock expectation on stream.Publish
	mockStream.On("Publish", mock.Anything, mock.MatchedBy(func(event *cloudevents.CloudEvent) bool {
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
	mockStream.AssertExpectations(t)
}
