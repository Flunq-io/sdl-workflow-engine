# Worker Service Event Processing Architecture

The Worker service implements a sophisticated event-driven workflow execution engine that processes workflow events using a robust, resilient pattern designed for enterprise-grade reliability.

## 🏗️ Core Architecture

### **Event Processing Pipeline**
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Event Stream  │───▶│  Event Filter   │───▶│ State Rebuild   │───▶│  Next Step      │
│  (Redis/Kafka)  │    │ & Validation    │    │  (Event Replay) │    │   Execution     │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
         ▲                        │                        │                        │
         │                        ▼                        ▼                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Event Ack     │    │ Execution-      │    │ Workflow Engine │    │ Task Publisher  │
│   & DLQ         │    │ Specific Filter │    │ (SDK Integration)│    │ & Timer Service │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🔄 Event Processing Sequence

### **1. Event Subscription & Consumer Groups**
The Worker service uses Redis Streams consumer groups for reliable event processing:

```go
filters := sharedinterfaces.StreamFilters{
    EventTypes: []string{
        "io.flunq.execution.started",
        "io.flunq.task.completed", 
        "io.flunq.timer.fired",
    },
    ConsumerGroup: "worker-service",
    ConsumerName:  "worker-{uuid}",
    BatchCount:    10,
    BlockTimeout:  time.Second,
}
```

**Key Features:**
- **Load Balancing**: Multiple worker instances share event processing load
- **Fault Tolerance**: Failed workers don't lose events due to consumer group semantics
- **Message Acknowledgment**: Proper ack/nack ensures exactly-once processing
- **Orphaned Message Recovery**: Background process reclaims pending messages

### **2. Event Filtering & Validation**
Events undergo multiple layers of filtering before processing:

#### **Source Filtering**
- Skips `io.flunq.workflow.created` events (handled by API service)
- Skips self-published `io.flunq.task.completed` events to prevent infinite loops
- Only processes `io.flunq.task.completed` from `executor-service` or `workflow-engine`

#### **Execution Isolation**
- Filters events by execution ID to prevent cross-execution contamination
- Ensures each workflow execution maintains isolated state
- Falls back to complete workflow history if execution ID missing

### **3. Execution-Specific Event History**
The Worker fetches and filters event history for precise state rebuilding:

```go
func (p *WorkflowProcessor) fetchEventHistoryForExecution(ctx context.Context, workflowID, executionID string) ([]*cloudevents.CloudEvent, error) {
    // Get all events for workflow
    allEvents, err := p.eventStream.GetEventHistory(ctx, workflowID)
    
    // Filter by execution ID for isolation
    var filteredEvents []*cloudevents.CloudEvent
    for _, event := range allEvents {
        if event.ExecutionID != "" && event.ExecutionID == executionID {
            filteredEvents = append(filteredEvents, event)
        }
    }
    return filteredEvents, nil
}
```

**Benefits:**
- **State Isolation**: Each execution maintains separate state
- **Deterministic Replay**: Exact state reconstruction from filtered events
- **Cross-Execution Protection**: Prevents state contamination between executions

### **4. Workflow Definition Retrieval**
Tenant-aware workflow definition lookup ensures proper context:

```go
definition, err := p.database.GetWorkflowDefinitionWithTenant(ctx, event.TenantID, event.WorkflowID)
```

**Features:**
- **Tenant Context**: Uses tenant ID from event for efficient lookup
- **Shared Database**: Leverages shared database interface for consistency
- **Error Handling**: Critical error logging for definition retrieval failures

### **5. Complete State Rebuilding**
The Worker rebuilds complete workflow state using the Serverless Workflow Go SDK:

```go
state, err := p.workflowEngine.RebuildState(ctx, definition, events)
```

**Process:**
- **Event Replay**: Processes filtered events in chronological order
- **SDK Integration**: Uses official Serverless Workflow SDK for state management
- **State Validation**: Ensures rebuilt state consistency
- **Debug Logging**: Detailed logging for troubleshooting

### **6. New Event Processing**
The newest event is applied to the rebuilt state:

```go
err := p.workflowEngine.ProcessEvent(ctx, state, event)
```

**Capabilities:**
- **Event Application**: Updates state based on event type and content
- **State Consistency**: Maintains workflow state integrity
- **Validation**: Ensures event processing follows DSL rules

### **7. Next Step Execution**
The Worker determines and executes the next workflow step:

```go
nextTask, err := p.workflowEngine.GetNextTask(ctx, state, definition)
```

**Task Type Handling:**
- **Wait Tasks**: Scheduled via timer service with event-driven timeouts
- **Regular Tasks**: Published as `io.flunq.task.requested` events
- **Set Tasks**: Handled internally with data manipulation
- **Completion Detection**: Supports both DSL 0.8 and 1.0.0 completion semantics

## 🛡️ Resilience Features

### **Concurrency Control**
- **Semaphore Limiting**: Configurable via `WORKER_CONCURRENCY` (default: 4)
- **Per-Workflow Locking**: Prevents concurrent processing of same workflow
- **Graceful Shutdown**: Waits for in-flight tasks to complete

### **Error Handling**
- **Retry Logic**: 3 attempts with exponential backoff (200ms * attempt)
- **Dead Letter Queue**: Failed events sent to DLQ after max retries
- **Circuit Breaker**: Resilient EventStore calls with failure handling

### **Message Reliability**
- **Acknowledgment**: Proper ack semantics with Redis Streams
- **Orphaned Recovery**: Background process reclaims pending messages
- **Exactly-Once Processing**: Consumer groups prevent duplicate processing

## ⏱️ Wait Task Processing

### **Event-Driven Timeouts**
Wait tasks use native event features for timing rather than polling:

```go
if task.TaskType == "wait" {
    duration, _ := time.ParseDuration(durationStr)
    err := p.waitScheduler.Schedule(ctx, tenantID, workflowID, executionID, taskName, duration)
}
```

**Flow:**
1. **Duration Parsing**: Supports ISO 8601 format (e.g., `PT2S`)
2. **Timer Scheduling**: Delegates to `WaitScheduler` abstraction
3. **Event Generation**: Timer service publishes `io.flunq.timer.fired`
4. **Workflow Resumption**: Worker processes timer event to continue

## 🔧 Configuration

### **Environment Variables**
- `WORKER_CONCURRENCY`: Concurrent event processors (default: 4)
- `WORKER_CONSUMER_GROUP`: Consumer group name (default: "worker-service")
- `WORKER_STREAM_BATCH`: Event batch size (default: 10)
- `WORKER_STREAM_BLOCK`: Block timeout (default: 1s)
- `WORKER_RECLAIM_ENABLED`: Orphaned message reclaim (default: false)

### **Backend Selection**
- `EVENTSTORE_TYPE`: redis, kafka, rabbitmq, nats
- `DB_TYPE`: redis, postgres, mongo, dynamo

## 📊 Monitoring & Observability

### **Metrics**
- `workflow_events_processed`: Counter by event type and workflow ID
- `workflow_event_processing_duration`: Timer by event type and workflow ID
- `workflow_event_processing_errors`: Counter by event type and attempt
- `workflow_subscription_errors`: Counter for subscription errors
- `workflow_event_dlq`: Counter for dead letter queue events

### **Logging**
- **Structured Logging**: JSON format with contextual fields
- **Debug Information**: Event data, state transitions, task execution
- **Error Tracking**: Detailed error context for troubleshooting
- **Performance Metrics**: Processing times and resource usage

## 🚀 Performance Characteristics

### **Throughput**
- **Concurrent Processing**: Configurable worker concurrency
- **Batch Processing**: Configurable batch sizes for event consumption
- **Efficient Filtering**: Execution-specific event filtering reduces processing overhead

### **Latency**
- **Direct Database Access**: Shared database interface for fast lookups
- **Event Sourcing**: Complete state rebuilding ensures consistency
- **SDK Integration**: Leverages optimized Serverless Workflow SDK

### **Scalability**
- **Horizontal Scaling**: Multiple worker instances with consumer groups
- **Stateless Design**: No local state, everything rebuilt from events
- **Pluggable Backends**: Easy scaling with different storage/streaming backends
