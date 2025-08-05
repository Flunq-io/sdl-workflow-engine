# I/O Storage System

## Overview

The Flunq.io workflow engine implements a comprehensive I/O storage system that captures and stores all input and output data for workflows and tasks. This enables complete workflow traceability, debugging, and replay capabilities.

## Current Implementation

### Data Serialization
- **Format**: JSON (not protobuf despite protobuf type definitions)
- **Storage**: Redis with JSON serialization
- **Type Safety**: Protobuf structs used for in-memory representation
- **Conversion**: `structpb.NewStruct()` converts Go maps to protobuf structs before JSON serialization

### Storage Locations

#### 1. Workflow State (`workflow:state:{workflow_id}`)
```json
{
  "id": "simple-test-workflow",
  "status": "COMPLETED",
  "input": {
    "message": "Hello from Multi-Tenant Postman!",
    "user_id": "test-user-456",
    "test_data": {
      "number": 42,
      "array": [1, 2, 3],
      "nested": {
        "key": "value",
        "deep": {
          "level": 2,
          "data": "nested_value"
        }
      }
    }
  },
  "output": {
    "result": "workflow completed successfully",
    "processed_data": {...}
  },
  "variables": {
    "temp_var": "some_value"
  },
  "completed_tasks": [
    {
      "task_name": "initialize",
      "input": {...},
      "output": {...},
      "started_at": "2024-01-15T14:30:00Z",
      "completed_at": "2024-01-15T14:30:01Z"
    }
  ]
}
```

#### 2. Event Streams (`events:workflow:{workflow_id}`)
Each event contains input/output data in the `data` field:

```json
{
  "event_type": "io.flunq.task.completed",
  "data": {
    "task_name": "processData",
    "input_data": {
      "source": "workflow_input",
      "parameters": {...}
    },
    "output_data": {
      "result": "processed successfully",
      "metrics": {...}
    }
  }
}
```

## Data Flow

### 1. Workflow Execution Started
```go
// Worker: serverless_workflow_engine.go:266
inputStruct, err := structpb.NewStruct(inputMap)
state.Input = inputStruct
```

### 2. Task Input Preparation
```go
// Worker: serverless_workflow_engine.go:543
taskInputStruct, err := structpb.NewStruct(taskInput)
taskData.Input = taskInputStruct
```

### 3. Task Output Storage
```go
// Executor processes task and returns output
// Worker stores output in task completion event
taskData.Output = outputStruct
```

### 4. Workflow Variables
```go
// Worker: serverless_workflow_engine.go:132
variablesStruct, err := structpb.NewStruct(variables)
state.Variables = variablesStruct
```

## UI Integration

### Event Timeline Enhancement
The UI event timeline displays I/O data with:

- **Color-coded sections**: Blue for inputs, green for outputs
- **Collapsible data display**: Click to expand/collapse JSON data
- **Field count badges**: Shows number of fields in each data object
- **Smart data extraction**: Automatically detects protobuf vs legacy JSON fields

### Data Extraction Logic
```typescript
function extractInputData(event: WorkflowEvent): Record<string, any> | null {
  // For workflow execution started events
  if (event.type.includes('execution.started') && event.data.input) {
    return event.data.input
  }
  
  // For task events - check for input_data (protobuf) or input
  if (event.type.includes('task.')) {
    if (event.data.input_data) return event.data.input_data
    if (event.data.input) return event.data.input
  }
  
  return null
}
```

## Storage Implementation Details

### Redis Database Adapter
```go
// flunq.io/worker/internal/adapters/redis_database.go:55
func (r *RedisDatabase) UpdateWorkflowState(ctx context.Context, workflowID string, state *gen.WorkflowState) error {
    // Serialize workflow state to JSON
    data, err := json.Marshal(state)
    if err != nil {
        return fmt.Errorf("failed to marshal workflow state: %w", err)
    }
    
    // Store in Redis with key: workflow:state:{id}
    key := fmt.Sprintf("workflow:state:%s", workflowID)
    if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
        return fmt.Errorf("failed to update workflow state: %w", err)
    }
    
    return nil
}
```

### Protobuf Serializer (Currently JSON)
```go
// flunq.io/worker/internal/serializer/protobuf_serializer.go:20
func (s *ProtobufSerializer) SerializeTaskData(data *gen.TaskData) ([]byte, error) {
    bytes, err := json.Marshal(data)  // ← JSON, not protobuf!
    if err != nil {
        return nil, fmt.Errorf("failed to serialize task data: %w", err)
    }
    return bytes, nil
}
```

## Benefits

### 1. Complete Traceability
- Every workflow execution is fully traceable
- All input/output data is preserved
- Task-level granularity for debugging

### 2. Replay Capabilities
- Workflows can be replayed with exact same inputs
- State reconstruction from event history
- Time-travel debugging

### 3. Monitoring & Analytics
- Performance analysis of task execution
- Data flow visualization
- Error pattern detection

### 4. Compliance & Auditing
- Complete audit trail of all data processing
- Regulatory compliance support
- Data lineage tracking

## Future Improvements

### 1. Protobuf Serialization
Replace JSON with binary protobuf for:
- **Performance**: Faster serialization/deserialization
- **Size**: Smaller storage footprint
- **Schema Evolution**: Better backward compatibility

### 2. Data Compression
- Compress large I/O payloads
- Configurable compression levels
- Automatic compression for data over threshold

### 3. Data Retention Policies
- Configurable retention periods
- Automatic cleanup of old data
- Archival to cold storage

### 4. Query Capabilities
- Search workflows by input/output content
- Advanced filtering and aggregation
- Real-time analytics dashboards

## Configuration

### Environment Variables
```bash
# Redis connection
REDIS_URL=redis://localhost:6379

# Storage settings
WORKFLOW_STATE_TTL=7d
EVENT_RETENTION_DAYS=30
MAX_PAYLOAD_SIZE=10MB
```

### Storage Limits
- **Max workflow input size**: 10MB (configurable)
- **Max task output size**: 5MB (configurable)
- **Event retention**: 30 days (configurable)
- **State retention**: 7 days (configurable)

## Troubleshooting

### Common Issues

1. **Large payloads**: Monitor payload sizes and implement compression
2. **Redis memory**: Monitor Redis memory usage and configure eviction policies
3. **Serialization errors**: Check for circular references in input data
4. **Performance**: Consider protobuf migration for high-throughput scenarios

### Monitoring

- Monitor Redis memory usage
- Track serialization/deserialization performance
- Alert on failed I/O storage operations
- Monitor payload size distribution
