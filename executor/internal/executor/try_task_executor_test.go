package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

// MockTaskExecutor for testing
type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) GetTaskType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
	args := m.Called(ctx, task)
	return args.Get(0).(*TaskResult), args.Error(1)
}

func TestTryTaskExecutor_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock executor
	mockExecutor := &MockTaskExecutor{}
	mockExecutor.On("GetTaskType").Return("call")

	// Setup successful execution
	successResult := &TaskResult{
		TaskID:   "test-task",
		Success:  true,
		Output:   map[string]interface{}{"result": "success"},
		Duration: time.Millisecond * 100,
	}
	mockExecutor.On("Execute", mock.Anything, mock.Anything).Return(successResult, nil)

	// Create try executor
	executors := map[string]TaskExecutor{"call": mockExecutor}
	tryExecutor := NewTryTaskExecutor(logger, executors)

	// Create try task
	task := &TaskRequest{
		TaskID:   "try-task-1",
		TaskType: "try",
		Config: &TaskConfig{
			Parameters: map[string]interface{}{
				"try": map[string]interface{}{
					"testCall": map[string]interface{}{
						"task_type": "call",
						"config": map[string]interface{}{
							"parameters": map[string]interface{}{
								"call": "http",
								"with": map[string]interface{}{
									"method":   "GET",
									"endpoint": "https://api.example.com",
								},
							},
						},
					},
				},
				"catch": map[string]interface{}{
					"errors": map[string]interface{}{
						"with": map[string]interface{}{
							"type": "https://serverlessworkflow.io/spec/1.0.0/errors/communication",
						},
					},
				},
			},
		},
	}

	// Execute
	result, err := tryExecutor.Execute(context.Background(), task)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "success", result.Output["result"])
	mockExecutor.AssertExpectations(t)
}

func TestTryTaskExecutor_RetryOnFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock executor
	mockExecutor := &MockTaskExecutor{}
	mockExecutor.On("GetTaskType").Return("call")

	// Setup failure then success
	failureResult := &TaskResult{
		TaskID:  "test-task",
		Success: false,
		Error:   "HTTP request failed with status 503",
	}
	successResult := &TaskResult{
		TaskID:   "test-task",
		Success:  true,
		Output:   map[string]interface{}{"result": "success"},
		Duration: time.Millisecond * 100,
	}

	mockExecutor.On("Execute", mock.Anything, mock.Anything).Return(failureResult, errors.New("HTTP request failed with status 503")).Once()
	mockExecutor.On("Execute", mock.Anything, mock.Anything).Return(successResult, nil).Once()

	// Create try executor
	executors := map[string]TaskExecutor{"call": mockExecutor}
	tryExecutor := NewTryTaskExecutor(logger, executors)

	// Create try task with retry policy
	task := &TaskRequest{
		TaskID:   "try-task-2",
		TaskType: "try",
		Config: &TaskConfig{
			Parameters: map[string]interface{}{
				"try": map[string]interface{}{
					"testCall": map[string]interface{}{
						"task_type": "call",
					},
				},
				"catch": map[string]interface{}{
					"errors": map[string]interface{}{
						"with": map[string]interface{}{
							"status": 503,
						},
					},
					"retry": map[string]interface{}{
						"delay": map[string]interface{}{
							"milliseconds": 100,
						},
						"backoff": map[string]interface{}{
							"constant": map[string]interface{}{},
						},
						"limit": map[string]interface{}{
							"attempt": map[string]interface{}{
								"count": 3,
							},
						},
					},
				},
			},
		},
	}

	// Execute
	result, err := tryExecutor.Execute(context.Background(), task)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "success", result.Output["result"])
	mockExecutor.AssertExpectations(t)
}

func TestTryTaskExecutor_RecoveryTasks(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock executors
	mockCallExecutor := &MockTaskExecutor{}
	mockCallExecutor.On("GetTaskType").Return("call")

	mockNotifyExecutor := &MockTaskExecutor{}
	mockNotifyExecutor.On("GetTaskType").Return("call")

	// Setup failure for main task
	failureResult := &TaskResult{
		TaskID:  "test-task",
		Success: false,
		Error:   "Payment failed with status 402",
	}
	mockCallExecutor.On("Execute", mock.Anything, mock.MatchedBy(func(task *TaskRequest) bool {
		return task.TaskID != "recovery-task"
	})).Return(failureResult, errors.New("Payment failed with status 402"))

	// Setup success for recovery task
	recoveryResult := &TaskResult{
		TaskID:   "recovery-task",
		Success:  true,
		Output:   map[string]interface{}{"notification": "sent"},
		Duration: time.Millisecond * 50,
	}
	mockNotifyExecutor.On("Execute", mock.Anything, mock.MatchedBy(func(task *TaskRequest) bool {
		return task.TaskID == "recovery-task"
	})).Return(recoveryResult, nil)

	// Create try executor
	executors := map[string]TaskExecutor{"call": mockCallExecutor}
	tryExecutor := NewTryTaskExecutor(logger, executors)

	// Create try task with recovery
	task := &TaskRequest{
		TaskID:   "try-task-3",
		TaskType: "try",
		Config: &TaskConfig{
			Parameters: map[string]interface{}{
				"try": map[string]interface{}{
					"processPayment": map[string]interface{}{
						"task_type": "call",
					},
				},
				"catch": map[string]interface{}{
					"errors": map[string]interface{}{
						"with": map[string]interface{}{
							"status": 402,
						},
					},
					"as": "paymentError",
					"do": map[string]interface{}{
						"notifyUser": map[string]interface{}{
							"task_type": "call",
							"task_id":   "recovery-task",
						},
					},
				},
			},
		},
	}

	// Execute
	result, err := tryExecutor.Execute(context.Background(), task)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "paymentError")
	mockCallExecutor.AssertExpectations(t)
}
