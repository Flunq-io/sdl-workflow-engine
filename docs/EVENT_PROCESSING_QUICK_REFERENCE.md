# Event Processing Quick Reference

This document provides a quick reference for understanding the Worker service event processing logic.

## 🔄 Event Processing Flow

```
Event Received → Filter & Validate → Fetch History → Rebuild State → Process Event → Execute Next → Update DB
      ↓               ↓                  ↓             ↓              ↓             ↓           ↓
  Consumer Group → Source Filter → Execution Filter → SDK Replay → State Update → Task/Timer → Persistence
```

## 📨 Event Types

### **Processed by Worker**
- ✅ `io.flunq.execution.started` - Start workflow execution
- ✅ `io.flunq.task.completed` - Task completion (executor-service only)
- ✅ `io.flunq.timer.fired` - Resume after wait task

### **Skipped by Worker**
- ❌ `io.flunq.workflow.created` - Handled by API service
- ❌ `io.flunq.task.completed` from worker-service - Prevents loops

### **Published by Worker**
- 📤 `io.flunq.task.requested` - Request task execution
- 📤 `io.flunq.workflow.completed` - Workflow finished
- 📤 `io.flunq.event.dlq` - Dead letter queue

## 🔧 Key Configuration

```bash
# Concurrency
WORKER_CONCURRENCY=4                    # Concurrent processors
WORKER_CONSUMER_GROUP=worker-service    # Consumer group name
WORKER_STREAM_BATCH=10                  # Event batch size
WORKER_STREAM_BLOCK=1s                  # Block timeout

# Resilience
WORKER_RECLAIM_ENABLED=false            # Orphaned message reclaim

# Backends
EVENTSTORE_TYPE=redis                   # Event streaming backend
DB_TYPE=redis                           # Database backend
```

## 🛡️ Resilience Features

### **Error Handling**
- **Retries**: 3 attempts with exponential backoff
- **DLQ**: Failed events sent to dead letter queue
- **Ack/Nack**: Proper message acknowledgment

### **Concurrency**
- **Semaphore**: Limits concurrent processors
- **Per-Workflow Locks**: Prevents concurrent processing of same workflow
- **Graceful Shutdown**: Waits for in-flight tasks

### **Message Reliability**
- **Consumer Groups**: Load balancing and fault tolerance
- **Orphaned Recovery**: Background reclaim of pending messages
- **Exactly-Once**: Prevents duplicate processing

## ⏱️ Wait Task Processing

```go
// Wait task detection
if task.TaskType == "wait" {
    // Parse duration (ISO 8601: PT2S = 2 seconds)
    duration, _ := time.ParseDuration(durationStr)
    
    // Schedule timer
    err := waitScheduler.Schedule(ctx, tenantID, workflowID, executionID, taskName, duration)
    
    // Timer service publishes io.flunq.timer.fired when duration expires
}
```

## 🔍 Event Filtering

### **Source Filtering**
```go
// Skip workflow.created (handled by API)
if event.Type == "io.flunq.workflow.created" {
    return nil // Skip
}

// Skip self-published task.completed (prevent loops)
if event.Source == "worker-service" && event.Type == "io.flunq.task.completed" {
    return nil // Skip
}

// Only process task.completed from executor-service
if event.Type == "io.flunq.task.completed" && event.Source != "executor-service" {
    return nil // Skip
}
```

### **Execution Filtering**
```go
// Filter events by execution ID for isolation
var filteredEvents []*cloudevents.CloudEvent
for _, event := range allEvents {
    if event.ExecutionID != "" && event.ExecutionID == executionID {
        filteredEvents = append(filteredEvents, event)
    }
}
```

## 📊 State Rebuilding

```go
// 1. Fetch execution-specific event history
events, err := fetchEventHistoryForExecution(ctx, workflowID, executionID)

// 2. Get workflow definition with tenant context
definition, err := database.GetWorkflowDefinitionWithTenant(ctx, tenantID, workflowID)

// 3. Rebuild state using Serverless Workflow SDK
state, err := workflowEngine.RebuildState(ctx, definition, events)

// 4. Process new event
err = workflowEngine.ProcessEvent(ctx, state, event)

// 5. Get next task
nextTask, err := workflowEngine.GetNextTask(ctx, state, definition)
```

## 🎯 Task Execution

### **Regular Tasks**
```go
// Publish task.requested event
event := &cloudevents.CloudEvent{
    Type:        "io.flunq.task.requested",
    Source:      "worker-service",
    WorkflowID:  workflowID,
    ExecutionID: executionID,
    Data: map[string]interface{}{
        "task_name": task.Name,
        "task_type": task.TaskType,
        "input":     task.Input,
    },
}
eventStream.Publish(ctx, event)
```

### **Wait Tasks**
```go
// Schedule timer instead of publishing task
waitScheduler.Schedule(ctx, tenantID, workflowID, executionID, taskName, duration)
```

## 📈 Monitoring

### **Key Metrics**
- `workflow_events_processed` - Events processed by type
- `workflow_event_processing_duration` - Processing time
- `workflow_event_processing_errors` - Processing errors
- `workflow_event_dlq` - Dead letter queue events

### **Log Fields**
```json
{
  "event_id": "event-123",
  "event_type": "io.flunq.task.completed",
  "workflow_id": "workflow-456",
  "execution_id": "execution-789",
  "tenant_id": "tenant-abc",
  "source": "executor-service",
  "processing_duration_ms": 150
}
```

## 🚨 Troubleshooting

### **Common Issues**

#### **Events Not Processing**
- Check consumer group configuration
- Verify event stream connectivity
- Check event filtering logic

#### **State Inconsistency**
- Verify execution ID filtering
- Check event history completeness
- Validate workflow definition

#### **Performance Issues**
- Adjust `WORKER_CONCURRENCY`
- Optimize batch sizes
- Monitor memory usage during state rebuilding

#### **Wait Tasks Not Resuming**
- Check timer service connectivity
- Verify `io.flunq.timer.fired` events
- Validate duration parsing

### **Debug Commands**
```bash
# Check Redis streams
redis-cli XINFO STREAM tenant:abc:events

# Check consumer groups
redis-cli XINFO GROUPS tenant:abc:events

# Check pending messages
redis-cli XPENDING tenant:abc:events worker-service
```

## 🔗 Related Documentation

- [Worker Service README](../worker/README.md) - Complete Worker service documentation
- [Event Store Architecture](EVENT_STORE_ARCHITECTURE.md) - EventStore design and implementation
- [Worker Event Processing](WORKER_EVENT_PROCESSING.md) - Detailed event processing architecture
- [Storage Architecture](architecture/storage-architecture.md) - Database and storage design
