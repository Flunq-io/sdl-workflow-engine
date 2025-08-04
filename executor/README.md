# Executor Service

The Executor Service is responsible for executing individual workflow tasks. It subscribes to `task.requested` events and publishes `task.completed` events.

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

### Output: `task.completed`
```json
{
  "id": "task-completed-task-initialize-123",
  "type": "io.flunq.task.completed",
  "source": "task-service",
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

## Running the Service

### Prerequisites
- Redis running on `localhost:6379`
- Event Store service running on `localhost:8081`

### Start the Service
```bash
cd executor
go run cmd/server/main.go
```

### Environment Variables
- `EVENTS_URL`: Event Store URL (default: `http://localhost:8081`)
- `REDIS_URL`: Redis URL (default: `localhost:6379`)
- `REDIS_PASSWORD`: Redis password (default: empty)
- `LOG_LEVEL`: Log level (default: `info`)

## Adding New Task Types

1. Create a new executor in `internal/executor/`
2. Implement the `TaskExecutor` interface
3. Register it in `cmd/server/main.go`

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
```

Register in main.go:
```go
taskExecutors := map[string]executor.TaskExecutor{
    "set":     executor.NewSetTaskExecutor(zapLogger),
    "call":    executor.NewCallTaskExecutor(zapLogger),
    "wait":    executor.NewWaitTaskExecutor(zapLogger),
    "inject":  executor.NewInjectTaskExecutor(zapLogger),
    "my_task": executor.NewMyTaskExecutor(zapLogger), // Add here
}
```
