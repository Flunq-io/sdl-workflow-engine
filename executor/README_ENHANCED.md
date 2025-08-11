# Executor Service

The Executor service handles task execution for the flunq.io workflow engine with enterprise-grade resilience. It processes task requests and executes various types of tasks including HTTP calls, data manipulation, and wait operations using sophisticated event processing patterns.

## 🚀 Features

### **Task Execution**
- ✅ **Multiple Task Types**: Supports call, set, wait, inject tasks
- ✅ **HTTP Integration**: External API calls with proper error handling
- ✅ **Data Processing**: Data transformation and manipulation tasks
- ✅ **Wait Operations**: Non-blocking delay execution

### **Enterprise-Grade Event Processing**
- ✅ **Consumer Groups**: Redis Streams consumer groups for load balancing
- ✅ **Retry Logic**: 3 attempts with exponential backoff (200ms * attempt)
- ✅ **Dead Letter Queue**: Failed events sent to DLQ after max retries
- ✅ **Message Acknowledgment**: Proper ack/nack semantics with Redis Streams
- ✅ **Orphaned Recovery**: Background process reclaims pending messages
- ✅ **Concurrency Control**: Configurable task concurrency with semaphore limiting
- ✅ **Graceful Shutdown**: Waits for in-flight tasks to complete

### **Resilience Features**
- ✅ **Circuit Breaker**: Resilient external API calls
- ✅ **Error Handling**: Comprehensive error logging and metrics
- ✅ **Health Checks**: Stream connectivity validation
- ✅ **Monitoring**: Structured logging with contextual information

## 🏗️ Architecture

The service follows a modular architecture with enhanced resilience:

```
executor/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── executor/        # Task execution logic
│   └── processor/       # Enhanced event processing with resilience
└── README.md
```

## 📨 Event Processing

### **Subscribed Events**
- `io.flunq.task.requested` - Task execution requests from Worker service

### **Published Events**
- `io.flunq.task.completed` - Task completion results
- `io.flunq.task.dlq` - Dead letter queue for failed tasks

### **Event Processing Flow**
```
Task Request → Retry Logic → Task Execution → Result Publishing → Message Ack
     ↓              ↓             ↓              ↓               ↓
Consumer Group → Exponential → Task Logic → Event Stream → Redis Streams
                 Backoff                                     Acknowledgment
```

### **Resilience Pattern**
1. **Event Subscription**: Consumer groups with load balancing
2. **Concurrency Control**: Semaphore-based task limiting
3. **Retry Logic**: 3 attempts with exponential backoff
4. **Error Handling**: DLQ for failed events after max retries
5. **Message Acknowledgment**: Proper ack semantics to prevent message loss
6. **Orphaned Recovery**: Background reclaim of pending messages

## 🎯 Task Types

### **Call Tasks**
- Execute HTTP requests to external APIs
- Support for GET, POST, PUT, DELETE methods
- Configurable timeouts and retries
- Response data extraction and transformation
- Circuit breaker protection

### **Set Tasks**
- Data manipulation and variable assignment
- JSON path operations
- Data transformation functions
- Context variable updates

### **Wait Tasks**
- Delay execution for specified duration
- Support for ISO 8601 duration format (e.g., PT2S for 2 seconds)
- Non-blocking implementation
- Immediate completion (actual wait handled by Timer service)

### **Inject Tasks**
- Data injection into workflow context
- Static data assignment
- Dynamic data generation
- Context enrichment

## 🔧 Configuration

### **Environment Variables**
- `EXECUTOR_CONCURRENCY`: Concurrent task processors (default: 4)
- `EXECUTOR_CONSUMER_GROUP`: Consumer group name (default: "executor-service")
- `EXECUTOR_STREAM_BATCH`: Event batch size (default: 10)
- `EXECUTOR_STREAM_BLOCK`: Block timeout (default: 1s)
- `EXECUTOR_RECLAIM_ENABLED`: Orphaned message reclaim (default: false)
- `REDIS_URL`: Redis connection string
- `EVENT_STREAM_TYPE`: Event stream backend (redis, kafka, etc.)
- `LOG_LEVEL`: Logging level

## 🚀 Quick Start

```bash
# Build the service
go build -o bin/executor cmd/server/main.go

# Run the service with enhanced resilience
./bin/executor

# Or run directly
go run cmd/server/main.go

# Run with custom configuration
EXECUTOR_CONCURRENCY=8 EXECUTOR_RECLAIM_ENABLED=true go run cmd/server/main.go
```

## 📊 Monitoring & Observability

### **Metrics**
- Task processing counters by type and status
- Processing duration timers
- Error counters by attempt and type
- Dead letter queue counters

### **Logging**
- Structured JSON logging with contextual fields
- Task execution details and timing
- Error tracking with full context
- Performance metrics and resource usage

## 🛡️ Error Handling

### **Retry Strategy**
- **Max Retries**: 3 attempts per task
- **Backoff**: Exponential (200ms * attempt number)
- **Failure Handling**: DLQ after max retries
- **Acknowledgment**: Proper message ack after DLQ

### **Circuit Breaker**
- Protection for external API calls
- Configurable failure thresholds
- Automatic recovery mechanisms
- Graceful degradation

### **Dead Letter Queue**
- Failed tasks sent to `io.flunq.task.dlq` events
- Includes failure reason and original event data
- Enables manual inspection and reprocessing
- Prevents message loss during failures

## 🔄 Deployment

### **Docker**
```bash
# Build Docker image
docker build -t flunq-executor .

# Run with environment variables
docker run -e REDIS_URL=redis://redis:6379 -e EXECUTOR_CONCURRENCY=8 flunq-executor
```

### **Kubernetes**
- Horizontal scaling with multiple replicas
- Consumer groups ensure load balancing
- Health checks for readiness and liveness
- Resource limits and requests configuration

## 🔗 Related Documentation

- [Worker Service README](../worker/README.md) - Complete Worker service documentation
- [Event Store Architecture](../docs/EVENT_STORE_ARCHITECTURE.md) - EventStore design and implementation
- [Event Processing Quick Reference](../docs/EVENT_PROCESSING_QUICK_REFERENCE.md) - Quick reference guide
