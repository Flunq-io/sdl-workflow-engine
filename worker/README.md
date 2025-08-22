# Worker Service

## ✅ **Status: Production Ready & Tested**

The Worker service executes Serverless Workflow DSL using the official Serverless Workflow Go SDK and implements the core event processing pattern for workflow orchestration with enterprise-grade resilience through the unified EventStore interface.

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
- ✅ **Enterprise-grade Resilience**: Crash recovery, horizontal scaling, time travel debugging

### **Core Event Processing Pattern**
Every workflow event triggers this exact sequence:

#### **1. Event Subscription & Filtering**
- **Consumer Groups**: Uses Redis Streams consumer groups for load balancing
- **Event Filtering**: Subscribes to specific event types:
  - `io.flunq.workflow.created` (skipped - handled by API service)
  - `io.flunq.execution.started`
  - `io.flunq.task.completed` (from executor-service or workflow-engine only)
  - `io.flunq.timer.fired` (for wait task resumption)
- **Tenant Isolation**: Automatic tenant-specific stream routing
- **Concurrency Control**: Configurable worker concurrency with semaphore-based limiting
- **Per-Workflow Locking**: Prevents concurrent processing of same workflow

#### **2. Execution-Specific Event History**
- **Execution Filtering**: Fetches events filtered by execution ID to isolate execution state
- **Cross-Execution Protection**: Strict filtering prevents contamination between executions
- **Fallback Handling**: Falls back to complete workflow history if execution ID missing
- **Event Sourcing**: Complete event history enables deterministic state rebuilding

#### **3. Workflow Definition Retrieval**
- **Tenant-Aware Lookup**: Uses tenant context from event for efficient definition retrieval
- **Shared Database**: Leverages shared database interface for consistent access
- **Error Handling**: Critical error logging for definition retrieval failures

#### **4. Complete State Rebuilding**
- **SDK Integration**: Uses Serverless Workflow Go SDK for state management
- **Event Replay**: Rebuilds complete workflow state from filtered event history
- **State Validation**: Ensures state consistency using SDK validation
- **Debug Logging**: Detailed logging of state rebuilding process

#### **5. New Event Processing**
- **Event Application**: Applies newest event to rebuilt state using workflow engine
- **State Updates**: Updates workflow state based on event type and content
- **Validation**: Ensures event processing maintains state consistency

#### **6. Next Step Execution**
- **DSL Interpretation**: Uses SDK to determine next action based on current state
- **Task Type Handling**: Different handling for various task types:
  - **Wait Tasks**: Scheduled via timer service with event-driven timeouts
  - **Regular Tasks**: Published as `io.flunq.task.requested` events for executor
  - **Set Tasks**: Handled internally with data manipulation
- **Completion Detection**: Detects workflow completion for both DSL 0.8 and 1.0.0 formats
- **Event Publishing**: Publishes appropriate events for next steps or completion

#### **7. Database Updates**
- **State Persistence**: Updates workflow state in database
- **Execution Tracking**: Updates execution status when workflow completes
- **Error Handling**: Graceful handling of database update failures

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

## 🔄 Event Processing Implementation

### **Event Loop Architecture**
The Worker service implements a robust event processing loop with the following characteristics:

#### **Consumer Group Semantics**
```go
// Event subscription with consumer groups
filters := sharedinterfaces.StreamFilters{
    EventTypes: []string{
        "io.flunq.workflow.created",
        "io.flunq.execution.started",
        "io.flunq.task.completed",
        "io.flunq.timer.fired",
    },
    ConsumerGroup: "worker-service",
    ConsumerName:  "worker-{uuid}",
    BatchCount:    10,
    BlockTimeout:  1 * time.Second,
}
```

#### **Concurrency Control**
- **Semaphore-based limiting**: Configurable via `WORKER_CONCURRENCY` (default: 4)
- **Per-workflow locking**: Prevents concurrent processing of same workflow
- **Graceful shutdown**: Waits for in-flight tasks to complete

#### **Error Handling & Resilience**
- **Retry Logic**: 3 attempts with exponential backoff (200ms * attempt)
- **Dead Letter Queue**: Failed events sent to DLQ after max retries
- **Message Acknowledgment**: Proper ack/nack semantics with Redis Streams
- **Orphaned Message Reclaim**: Background process reclaims pending messages

#### **Event Filtering & Routing**
- **Source Filtering**: Skips self-published events to prevent infinite loops
- **Execution Isolation**: Filters events by execution ID for state isolation
- **Tenant Routing**: Automatic routing to tenant-specific streams

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

### **Wait Task Processing**
The Worker service implements sophisticated wait task handling using event-driven timeouts:

#### **Timer Service Integration**
- **Event-Driven Waits**: Wait tasks use native event features rather than polling
- **Timer Scheduling**: Delegates to `WaitScheduler` abstraction for timeout management
- **Timeout Events**: Generates `io.flunq.timer.fired` events when wait duration expires
- **Duration Parsing**: Supports ISO 8601 duration format (e.g., `PT2S` for 2 seconds)

#### **Wait Task Flow**
1. **Task Detection**: Identifies wait tasks by `task_type: "wait"`
2. **Duration Extraction**: Parses duration from task input
3. **Timer Scheduling**: Schedules timeout via timer service
4. **Event Generation**: Timer service publishes `io.flunq.timer.fired` event
5. **Workflow Resumption**: Worker processes timer event to continue workflow

```go
// Wait task scheduling example
if task.TaskType == "wait" {
    duration, _ := time.ParseDuration(durationStr)
    err := p.waitScheduler.Schedule(ctx, tenantID, workflowID, executionID, taskName, duration)
}
```

## 📊 **Configuration**

Configuration is managed through environment variables. Copy `.env.example` to `.env` and customize:

### **Configuration Options**

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `localhost:6379` | Redis connection URL |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PASSWORD` | `` | Redis password (if required) |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `HEALTH_PORT` | `8082` | Health check endpoint port |
| `WORKER_CONCURRENCY` | `4` | Number of concurrent event processors |
| `WORKER_CONSUMER_GROUP` | `worker-service` | Consumer group name |
| `WORKER_STREAM_BATCH` | `10` | Batch size for event consumption |
| `WORKER_STREAM_BLOCK` | `1s` | Block timeout for event consumption |
| `WORKER_RECLAIM_ENABLED` | `false` | Enable orphaned message reclaim |
| `EVENTSTORE_TYPE` | `redis` | EventStore backend type |
| `DB_TYPE` | `redis` | Database backend type |
| `TIMER_SERVICE_ENABLED` | `true` | Enable timer service |
| `TIMER_SERVICE_PRECISION` | `1s` | Timer service precision |
| `WORKFLOW_ENGINE_TYPE` | `serverless` | Workflow engine type |
| `WORKFLOW_TIMEOUT_SECONDS` | `3600` | Workflow execution timeout |
| `WORKFLOW_MAX_RETRIES` | `3` | Workflow maximum retries |
| `EVENT_BATCH_SIZE` | `10` | Event processing batch size |
| `EVENT_PROCESSING_TIMEOUT` | `30s` | Event processing timeout |
| `EVENT_RETRY_ATTEMPTS` | `3` | Event processing retry attempts |
| `EVENT_RETRY_DELAY_MS` | `200` | Event retry delay in milliseconds |

## 🚀 **Quick Start**

### Prerequisites
- Redis running on `localhost:6379`
- Go 1.21 or later

### Setup and Run
```bash
# Configure environment
cp .env.example .env
# Edit .env with your settings

# Build and run
make build
make run

# Or run in development mode
make dev

# Run tests
make test

# Run with coverage
make coverage

# Build Docker image
make docker-build
make docker-run

# Test all components
go test ./... -v

# Run example (requires Redis)
go run examples/worker-example.go
```

## 📨 Event Types & Processing

### **Subscribed Events**
The Worker service subscribes to and processes these event types:

#### **Execution Events**
- `io.flunq.execution.started` - Triggers workflow execution start
- `io.flunq.task.completed` - Processes task completion (from executor-service only)
- `io.flunq.timer.fired` - Resumes workflows after wait tasks

#### **Skipped Events**
- `io.flunq.workflow.created` - Skipped (handled by API service)
- `io.flunq.task.completed` from non-executor sources - Prevents infinite loops

#### **Published Events**
The Worker service publishes these events during processing:

- `io.flunq.task.requested` - Requests task execution from executor service
- `io.flunq.workflow.completed` - Signals workflow completion
- `io.flunq.event.dlq` - Dead letter queue for failed events

### **Event Processing Flow**
```
Incoming Event → Execution Filtering → State Rebuild → Event Processing → Next Step → Database Update
      ↓                    ↓                ↓              ↓             ↓            ↓
   Tenant Route    →  History Fetch  →  SDK Replay  →  State Update → Task/Timer → Persistence
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
