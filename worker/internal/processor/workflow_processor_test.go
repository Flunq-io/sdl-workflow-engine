package processor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/flunq-io/shared/pkg/cloudevents"
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/worker/internal/interfaces"
	"github.com/flunq-io/worker/proto/gen"
)

// FakeEventStream is a simple test double for the shared EventStream
// It only implements the methods we need for this unit test
// (GetEventHistory and no-op Publish/Close)
type FakeEventStream struct {
	Events []*cloudevents.CloudEvent
}

// Use the shared interfaces package for proper typing
// Note: Subscribe/CreateConsumerGroup/DeleteConsumerGroup are not used in this test
func (f *FakeEventStream) Subscribe(ctx context.Context, filters sharedinterfaces.StreamFilters) (sharedinterfaces.StreamSubscription, error) {
	return nil, nil
}

func (f *FakeEventStream) Publish(ctx context.Context, event *cloudevents.CloudEvent) error {
	return nil
}

func (f *FakeEventStream) GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error) {
	return f.Events, nil
}

func (f *FakeEventStream) CreateConsumerGroup(ctx context.Context, groupName string) error {
	return nil
}
func (f *FakeEventStream) DeleteConsumerGroup(ctx context.Context, groupName string) error {
	return nil
}
func (f *FakeEventStream) GetStreamInfo(ctx context.Context) (*sharedinterfaces.StreamInfo, error) {
	return &sharedinterfaces.StreamInfo{}, nil
}
func (f *FakeEventStream) ReclaimPending(ctx context.Context, consumerGroup, consumerName string, minIdle time.Duration) (int, error) {
	return 0, nil
}

func (f *FakeEventStream) Close() error { return nil }

// MockDatabase is a mock implementation of Database interface
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) CreateWorkflow(ctx context.Context, workflow *gen.WorkflowDefinition) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockDatabase) UpdateWorkflowState(ctx context.Context, workflowID string, state *gen.WorkflowState) error {
	args := m.Called(ctx, workflowID, state)
	return args.Error(0)
}

func (m *MockDatabase) GetWorkflowState(ctx context.Context, workflowID string) (*gen.WorkflowState, error) {
	args := m.Called(ctx, workflowID)
	return args.Get(0).(*gen.WorkflowState), args.Error(1)
}

func (m *MockDatabase) GetWorkflowDefinition(ctx context.Context, workflowID string) (*gen.WorkflowDefinition, error) {
	args := m.Called(ctx, workflowID)
	return args.Get(0).(*gen.WorkflowDefinition), args.Error(1)
}

func (m *MockDatabase) GetWorkflowDefinitionWithTenant(ctx context.Context, tenantID, workflowID string) (*gen.WorkflowDefinition, error) {
	args := m.Called(ctx, tenantID, workflowID)
	return args.Get(0).(*gen.WorkflowDefinition), args.Error(1)
}

func (m *MockDatabase) DeleteWorkflow(ctx context.Context, workflowID string) error {
	args := m.Called(ctx, workflowID)
	return args.Error(0)
}

func (m *MockDatabase) ListWorkflows(ctx context.Context, filters map[string]string) ([]*gen.WorkflowState, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*gen.WorkflowState), args.Error(1)
}

// MockWorkflowEngine is a mock implementation of WorkflowEngine interface
type MockWorkflowEngine struct {
	mock.Mock
}

func (m *MockWorkflowEngine) ParseDefinition(ctx context.Context, dslJSON []byte) (*gen.WorkflowDefinition, error) {
	args := m.Called(ctx, dslJSON)
	return args.Get(0).(*gen.WorkflowDefinition), args.Error(1)
}

func (m *MockWorkflowEngine) ValidateDefinition(ctx context.Context, definition *gen.WorkflowDefinition) error {
	args := m.Called(ctx, definition)
	return args.Error(0)
}

func (m *MockWorkflowEngine) InitializeState(ctx context.Context, definition *gen.WorkflowDefinition, input map[string]interface{}) (*gen.WorkflowState, error) {
	args := m.Called(ctx, definition, input)
	return args.Get(0).(*gen.WorkflowState), args.Error(1)
}

func (m *MockWorkflowEngine) RebuildState(ctx context.Context, definition *gen.WorkflowDefinition, events []*cloudevents.CloudEvent) (*gen.WorkflowState, error) {
	args := m.Called(ctx, definition, events)
	return args.Get(0).(*gen.WorkflowState), args.Error(1)
}

func (m *MockWorkflowEngine) GetNextTask(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) (*gen.PendingTask, error) {
	args := m.Called(ctx, state, definition)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gen.PendingTask), args.Error(1)
}

func (m *MockWorkflowEngine) ProcessEvent(ctx context.Context, state *gen.WorkflowState, event *cloudevents.CloudEvent) error {
	args := m.Called(ctx, state, event)
	return args.Error(0)
}

func (m *MockWorkflowEngine) ExecuteTask(ctx context.Context, task *gen.PendingTask, state *gen.WorkflowState) (*gen.TaskData, error) {
	args := m.Called(ctx, task, state)
	return args.Get(0).(*gen.TaskData), args.Error(1)
}

func (m *MockWorkflowEngine) IsWorkflowComplete(ctx context.Context, state *gen.WorkflowState, definition *gen.WorkflowDefinition) bool {
	args := m.Called(ctx, state, definition)
	return args.Bool(0)
}

// MockSerializer is a mock implementation of ProtobufSerializer interface
type MockSerializer struct {
	mock.Mock
}

func (m *MockSerializer) SerializeTaskData(data *gen.TaskData) ([]byte, error) {
	args := m.Called(data)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSerializer) DeserializeTaskData(data []byte) (*gen.TaskData, error) {
	args := m.Called(data)
	return args.Get(0).(*gen.TaskData), args.Error(1)
}

func (m *MockSerializer) SerializeWorkflowState(state *gen.WorkflowState) ([]byte, error) {
	args := m.Called(state)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSerializer) DeserializeWorkflowState(data []byte) (*gen.WorkflowState, error) {
	args := m.Called(data)
	return args.Get(0).(*gen.WorkflowState), args.Error(1)
}

func (m *MockSerializer) SerializeTaskRequestedEvent(event *gen.TaskRequestedEvent) ([]byte, error) {
	args := m.Called(event)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSerializer) DeserializeTaskRequestedEvent(data []byte) (*gen.TaskRequestedEvent, error) {
	args := m.Called(data)
	return args.Get(0).(*gen.TaskRequestedEvent), args.Error(1)
}

func (m *MockSerializer) SerializeTaskCompletedEvent(event *gen.TaskCompletedEvent) ([]byte, error) {
	args := m.Called(event)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSerializer) DeserializeTaskCompletedEvent(data []byte) (*gen.TaskCompletedEvent, error) {
	args := m.Called(data)
	return args.Get(0).(*gen.TaskCompletedEvent), args.Error(1)
}

// MockLogger is a mock implementation of Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...interface{}) interfaces.Logger {
	args := m.Called(fields)
	return args.Get(0).(interfaces.Logger)
}

// MockMetrics is a mock implementation of Metrics interface
type MockMetrics struct {
	mock.Mock
}

func (m *MockMetrics) IncrementCounter(name string, tags map[string]string) {
	m.Called(name, tags)
}

func (m *MockMetrics) RecordHistogram(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMetrics) RecordGauge(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMetrics) StartTimer(name string, tags map[string]string) func() {
	args := m.Called(name, tags)
	return args.Get(0).(func())
}

func TestWorkflowProcessor_ProcessWorkflowEvent(t *testing.T) {
	// Setup mocks
	mockDatabase := &MockDatabase{}
	mockEngine := &MockWorkflowEngine{}
	// Serializer is not used directly in this test; avoid unused var errors
	_ = &MockSerializer{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetrics{}

	ctx := context.Background()
	workflowID := "test-workflow-123"

	// Create test event
	event := &cloudevents.CloudEvent{
		ID:          "event-123",
		Type:        "io.flunq.execution.started",
		WorkflowID:  workflowID,
		ExecutionID: "exec-123",
		Time:        time.Now(),
		Data: map[string]interface{}{
			"correlation_id": "corr-123",
			"input": map[string]interface{}{
				"user_id": "user-456",
			},
		},
	}

	// Create test workflow definition
	dslDefinition, _ := structpb.NewStruct(map[string]interface{}{
		"id":          workflowID,
		"name":        "Test Workflow",
		"specVersion": "0.8",
		"start":       "hello",
		"states": []interface{}{
			map[string]interface{}{
				"name": "hello",
				"type": "inject",
				"data": map[string]interface{}{
					"message": "Hello World",
				},
				"end": true,
			},
		},
	})

	definition := &gen.WorkflowDefinition{
		Id:            workflowID,
		Name:          "Test Workflow",
		SpecVersion:   "0.8",
		StartState:    "hello",
		DslDefinition: dslDefinition,
	}

	// Create test workflow state
	state := &gen.WorkflowState{
		WorkflowId:  workflowID,
		CurrentStep: "hello",
		Status:      gen.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		Context: &gen.ExecutionContext{
			ExecutionId:   "exec-123",
			CorrelationId: "corr-123",
		},
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
	}

	// Setup mock expectations
	mockMetrics.On("StartTimer", "workflow_event_processing_duration", mock.AnythingOfType("map[string]string")).Return(func() {})
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).Return().Maybe()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).Return().Maybe()
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).Return().Maybe()
	mockDatabase.On("GetWorkflowDefinitionWithTenant", ctx, "", workflowID).Return(definition, nil).Maybe()
	mockDatabase.On("GetWorkflowDefinition", ctx, workflowID).Return(definition, nil).Maybe()
	mockEngine.On("RebuildState", ctx, definition, mock.AnythingOfType("[]*cloudevents.CloudEvent")).Return(state, nil)
	mockEngine.On("ProcessEvent", ctx, state, event).Return(nil)
	mockEngine.On("IsWorkflowComplete", ctx, state, definition).Return(false)
	mockEngine.On("GetNextTask", ctx, state, definition).Return(nil, nil) // No next task
	mockDatabase.On("UpdateWorkflowState", ctx, workflowID, state).Return(nil)
	mockMetrics.On("IncrementCounter", "workflow_events_processed", mock.AnythingOfType("map[string]string")).Return()

	// Execute using a minimal WorkflowProcessor instance with our fakes/mocks
	proc := &WorkflowProcessor{
		eventStream:    &FakeEventStream{Events: []*cloudevents.CloudEvent{event}},
		database:       mockDatabase,
		sharedDatabase: nil,
		workflowEngine: mockEngine,
		serializer:     nil,
		logger:         mockLogger,
		metrics:        mockMetrics,
		workflowLocks:  make(map[string]*sync.Mutex),
	}
	err := proc.ProcessWorkflowEvent(ctx, event)

	// Assert
	assert.NoError(t, err)
	mockDatabase.AssertExpectations(t)
	mockEngine.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}
