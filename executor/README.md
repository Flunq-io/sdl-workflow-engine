# Executor Service

The Executor Service is responsible for executing individual workflow tasks with enterprise-grade error handling and retry capabilities. It subscribes to `task.requested` events and publishes `task.completed` events with proper status tracking.

## Architecture

```
Worker Service → task.requested → Executor Service → task.completed → Worker Service
```

## Supported Task Types

### 1. Set Task (`set`)
Sets variables in the workflow context.

**Example Configuration:**
```yaml
- name: initialize
  type: set
  config:
    parameters:
      user_id: "12345"
      status: "active"
      timestamp: "2024-01-01T00:00:00Z"
```

### 2. Call Task (`call`)
Makes HTTP/API calls to external services.

**Example Configuration:**
```yaml
- name: fetch_user_data
  type: call
  config:
    parameters:
      url: "https://api.example.com/users/12345"
      method: "GET"
      headers:
        Authorization: "Bearer token123"
        Content-Type: "application/json"
```

### 3. Wait Task (`wait`)
Waits for a specified duration.

**Example Configuration:**
```yaml
- name: delay_step
  type: wait
  config:
    parameters:
      duration: "5s"  # or use "seconds": 5
```

### 4. Inject Task (`inject`)
Injects data/variables into the workflow context.

**Example Configuration:**
```yaml
- name: inject_metadata
  type: inject
  config:
    parameters:
      environment: "production"
      region: "us-east-1"
      version: "1.0.0"
```

### 5. Try Task (`try`) - SDL Error Handling
Executes tasks with sophisticated error handling, retry policies, and recovery mechanisms.

**Basic Try/Catch Example:**
```yaml
- name: api_call_with_retry
  type: try
  config:
    parameters:
      try:
        callExternalAPI:
          task_type: call
          config:
            parameters:
              call: http
              with:
                method: get
                endpoint: https://api.example.com/data
                headers:
                  Authorization: "Bearer ${.token}"
      catch:
        errors:
          with:
            type: "https://serverlessworkflow.io/spec/1.0.0/errors/communication"
            status: 503
        retry:
          delay:
            seconds: 2
          backoff:
            exponential:
              multiplier: 2.0
              maxDelay:
                seconds: 30
          limit:
            attempt:
              count: 5
```

**Error Recovery Example:**
```yaml
- name: payment_with_recovery
  type: try
  config:
    parameters:
      try:
        processPayment:
          task_type: call
          config:
            parameters:
              call: http
              with:
                method: post
                endpoint: https://payment.service.com/charge
                body:
                  amount: "${.order.total}"
                  currency: "USD"
      catch:
        errors:
          with:
            status: 402  # Payment Required
        as: "paymentError"
        do:
          notifyCustomer:
            task_type: call
            config:
              parameters:
                call: http
                with:
                  method: post
                  endpoint: https://notification.service.com/send
                  body:
                    message: "Payment failed: ${.paymentError.detail}"
```

## Event Flow

### Input: `task.requested`
```json
{
  "id": "task-req-123",
  "type": "io.flunq.task.requested",
  "source": "worker-service",
  "workflow_id": "simple-test-workflow",
  "data": {
    "task_id": "task-initialize-123",
    "task_name": "initialize",
    "task_type": "set",
    "config": {
      "parameters": {
        "user_id": "12345"
      }
    }
  }
}
```

### Output: `task.completed` (Success)
```json
{
  "id": "task-completed-task-initialize-123",
  "type": "io.flunq.task.completed",
  "source": "executor-service",
  "workflow_id": "simple-test-workflow",
  "data": {
    "task_id": "task-initialize-123",
    "task_name": "initialize",
    "success": true,
    "output": {
      "user_id": "12345"
    },
    "duration_ms": 15,
    "executed_at": "2024-01-01T00:00:00Z"
  }
}
```

### Output: `task.completed` (Failed)
```json
{
  "id": "task-completed-task-api-call-456",
  "type": "io.flunq.task.completed",
  "source": "executor-service",
  "workflow_id": "simple-test-workflow",
  "data": {
    "task_id": "task-api-call-456",
    "task_name": "callAPI",
    "success": false,
    "error": "API call failed with status 503: Service Unavailable",
    "output": {},
    "duration_ms": 5000,
    "executed_at": "2024-01-01T00:00:00Z"
  }
}
```

## SDL Error Handling Features

### Retry Policies
The executor supports sophisticated retry policies with multiple backoff strategies:

- **Constant Backoff**: Fixed delay between retries
- **Linear Backoff**: Linearly increasing delay
- **Exponential Backoff**: Exponentially increasing delay with optional jitter
- **Jitter Support**: Random delay variation to prevent thundering herd

### Error Filtering
Errors can be filtered by:
- **Error Type**: SDL-compliant error type URIs
- **HTTP Status**: Specific status codes or ranges
- **Custom Properties**: Any error property using pattern matching
- **Conditional Logic**: Runtime expressions for complex filtering

### Error Recovery
When errors are caught, you can:
- **Execute Recovery Tasks**: Run specific tasks to handle the error
- **Store Error Context**: Access error details in recovery tasks
- **Continue or Fail**: Choose whether to continue or fail the workflow

### Status Tracking
The executor provides comprehensive status tracking:

#### Task Status
- `TASK_STATUS_PENDING`: Task queued for execution
- `TASK_STATUS_RUNNING`: Task currently executing
- `TASK_STATUS_COMPLETED`: Task finished successfully
- `TASK_STATUS_FAILED`: Task failed after retries exhausted
- `TASK_STATUS_CANCELLED`: Task cancelled by user
- `TASK_STATUS_RETRYING`: Task currently retrying

#### Workflow Status
- `WORKFLOW_STATUS_RUNNING`: Workflow executing normally
- `WORKFLOW_STATUS_FAILED`: Workflow failed due to task failure
- `WORKFLOW_STATUS_COMPLETED`: Workflow finished successfully

When a task fails after exhausting all retry attempts:
1. Task status is set to `TASK_STATUS_FAILED`
2. Workflow status is set to `WORKFLOW_STATUS_FAILED`
3. Error information is stored in workflow variables
4. No further tasks are processed

## Running the Service

### Prerequisites
- Redis running on `localhost:6379`

### Start the Service
```bash
cd executor
go run cmd/server/main.go
```

### Environment Variables
- `EVENT_STREAM_TYPE`: Event backend (default: `redis`)
- `REDIS_URL`: Redis URL (default: `localhost:6379`)
- `REDIS_PASSWORD`: Redis password (default: empty)
- `LOG_LEVEL`: Log level (default: `info`)

## Adding New Task Types

1. Create a new executor in `internal/executor/`
2. Implement the `TaskExecutor` interface
3. Register it in the `ExecutorRegistry`

Example:
```go
// internal/executor/my_task_executor.go
type MyTaskExecutor struct {
    logger *zap.Logger
}

func (e *MyTaskExecutor) Execute(ctx context.Context, task *TaskRequest) (*TaskResult, error) {
    // Implementation here
}

func (e *MyTaskExecutor) GetTaskType() string {
    return "my_task"
}

func NewMyTaskExecutor(logger *zap.Logger) TaskExecutor {
    return &MyTaskExecutor{logger: logger}
}
```

Register in the executor registry:
```go
// internal/executor/executor_registry.go
func NewExecutorRegistry(logger *zap.Logger) *ExecutorRegistry {
    registry := &ExecutorRegistry{
        executors: make(map[string]TaskExecutor),
        logger:    logger,
    }

    // Register all task executors
    registry.Register(NewSetTaskExecutor(logger))
    registry.Register(NewCallTaskExecutor(logger))
    registry.Register(NewWaitTaskExecutor(logger))
    registry.Register(NewInjectTaskExecutor(logger))
    registry.Register(NewTryTaskExecutor(logger))
    registry.Register(NewMyTaskExecutor(logger)) // Add here

    return registry
}
```

The executor registry automatically manages all task executors and provides centralized access. The main.go file now uses the registry instead of hardcoded executor maps:

```go
// cmd/server/main.go
taskProcessor := processor.NewTaskProcessor(
    eventStream,
    zapLogger,
)
```
