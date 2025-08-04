# Worker Service

The Worker service executes workflow definitions using the Serverless Workflow DSL.

## 🏗️ Architecture

```
┌─────────────────┐
│  DSL Executor   │
│  (State Machine)│
└─────────┬───────┘
          │
┌─────────▼───────┐
│  State Manager  │
│  (Redis/Memory) │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Task Scheduler │
│  (Queue/Events) │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Event Consumer │
│  (Redis/Kafka)  │
└─────────────────┘
```

## 🚀 Features

- **DSL Execution**: Serverless Workflow Definition Language interpreter
- **State Management**: Workflow state persistence and recovery
- **Task Scheduling**: Parallel and sequential task execution
- **Error Handling**: Retry policies, timeouts, and compensation
- **Event-Driven**: React to workflow and task events
- **Scalability**: Horizontal scaling with worker pools

## 📁 Structure

```
worker/
├── cmd/
│   └── worker/
│       └── main.go
├── internal/
│   ├── executor/
│   ├── state/
│   ├── scheduler/
│   └── handlers/
├── pkg/
│   ├── dsl/
│   ├── workflow/
│   └── tasks/
├── configs/
├── Dockerfile
├── go.mod
└── README.md
```

## 🔧 Configuration

Environment variables:
- `WORKER_ID`: Unique worker identifier
- `CONCURRENCY`: Number of concurrent workflows (default: 10)
- `REDIS_URL`: Redis connection string
- `EVENTS_URL`: Events service URL
- `EXECUTOR_URL`: Executor service URL
- `LOG_LEVEL`: Logging level

## 🚀 Quick Start

```bash
# Install dependencies
go mod tidy

# Run locally
go run cmd/worker/main.go

# Build
go build -o bin/worker cmd/worker/main.go

# Run with Docker
docker build -t flunq-worker .
docker run flunq-worker
```

## 🔄 Workflow States

### Execution States
- `PENDING` - Workflow queued for execution
- `RUNNING` - Workflow currently executing
- `COMPLETED` - Workflow finished successfully
- `FAILED` - Workflow failed with error
- `CANCELLED` - Workflow cancelled by user
- `PAUSED` - Workflow paused for manual intervention

### Task States
- `SCHEDULED` - Task queued for execution
- `RUNNING` - Task currently executing
- `COMPLETED` - Task finished successfully
- `FAILED` - Task failed (may retry)
- `SKIPPED` - Task skipped due to conditions

## 📋 Supported DSL Features

### States
- **Operation State**: Execute actions/functions
- **Switch State**: Conditional branching
- **Parallel State**: Concurrent execution
- **ForEach State**: Iterate over data
- **Inject State**: Data manipulation
- **Sleep State**: Delays and timeouts
- **Event State**: Wait for events

### Actions
- **Function Calls**: Invoke external services
- **Subflow Calls**: Execute nested workflows
- **Event Publishing**: Emit events

### Error Handling
- **Retry Policies**: Exponential backoff, max attempts
- **Compensation**: Rollback actions
- **Error Events**: Publish error events
