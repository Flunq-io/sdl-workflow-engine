# Worker Service

The Worker service executes Serverless Workflow DSL using the official Serverless Workflow Go SDK and implements the core event processing pattern for workflow orchestration with **Temporal-level resilience** through the unified EventStore interface.

## 🚀 Features

### **Serverless Workflow Integration**
- ✅ **Official SDK**: Uses `github.com/serverlessworkflow/sdk-go` for DSL parsing and execution
- ✅ **SDL Task Types**: Supports call, run, for, if, switch, try, emit, wait, set tasks
- ✅ **SDK Validation**: Leverages SDK's built-in validation and workflow model
- ✅ **State Management**: Uses SDK's workflow execution engine as foundation

### **EventStore Integration**
- ✅ **Unified Interface**: Uses pluggable EventStore interface for maximum flexibility
- ✅ **Redis Implementation**: Current backend using Redis Streams with consumer groups
- ✅ **Future Backends**: Easy switching to Kafka, RabbitMQ, PostgreSQL via configuration
- ✅ **Event Sourcing**: Complete event history with deterministic state rebuilding
- ✅ **Temporal-level Resilience**: Crash recovery, horizontal scaling, time travel debugging

### **Core Event Processing Pattern**
Every workflow event triggers this exact sequence:
1. **Fetch Complete Event History** - Get ALL events from beginning
2. **Rebuild Complete Workflow State** - Replay events using SDK state management
3. **Process New Event** - Apply newest event to rebuilt state
4. **Execute Next SDL Step** - Use SDK to determine and execute next action
5. **Update Workflow Record** - Persist state changes to database

### **Enhanced I/O Storage System**
- ✅ **JSON Serialization**: Complete workflow and task I/O data stored as JSON
- ✅ **Protobuf Type Safety**: Protobuf structs for in-memory type safety
- ✅ **Complete Traceability**: All input/output data preserved for debugging
- ✅ **UI Integration**: Real-time I/O data visualization in event timeline
- 🚧 **Binary Protobuf**: Planned migration to binary protobuf for performance

### **Generic Interface Architecture**
- ✅ **EventStore Interface**: Switch between Redis, Kafka, RabbitMQ, PostgreSQL
- ✅ **Database Interface**: Switch between Redis, PostgreSQL, MongoDB
- ✅ **WorkflowEngine Interface**: Pluggable workflow execution engines
- ✅ **Circuit Breaker**: Resilient EventStore calls with failure handling
- ✅ **Configuration-Driven**: Backend selection via environment variables

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
- `EVENTSTORE_TYPE`: EventStore backend type (redis, kafka, rabbitmq)
- `EXECUTOR_URL`: Executor service URL
- `LOG_LEVEL`: Logging level

## 🚀 Quick Start

```bash
# Install dependencies
go mod tidy

# Build the service
go build -o bin/worker cmd/server/main.go

# Run the service (requires Redis)
./bin/worker
go run cmd/server/main.go

# Run tests
go test ./internal/processor -v

# Test all components
go test ./... -v

# Run example (requires Redis)
go run examples/worker-example.go
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
