# Storage Architecture

## Overview

The Flunq.io platform uses a **centralized storage architecture** with generic interfaces that can be implemented for different storage backends (Redis, PostgreSQL, MongoDB, etc.). This ensures consistency, eliminates data duplication, and provides flexibility for future storage migrations.

## Architecture Principles

### 1. **Single Source of Truth**
- Each workflow definition is stored **once** using the workflow instance ID as the key
- All services access the same centralized storage through shared interfaces
- No duplicate storage across different services

### 2. **Generic Interfaces**
- Storage operations are abstracted through generic interfaces
- Multiple storage backends can be implemented (Redis, PostgreSQL, MongoDB)
- Easy to swap storage implementations without changing business logic

### 3. **Tenant Isolation**
- All storage operations support multi-tenant isolation
- Tenant ID is included in all workflow definitions and queries
- Stream keys and storage keys include tenant information

## Storage Interfaces

### Database Interface (`interfaces.Database`)

The main interface for workflow metadata storage:

```go
type Database interface {
    // Workflow operations
    CreateWorkflow(ctx context.Context, workflow *WorkflowDefinition) error
    GetWorkflowDefinition(ctx context.Context, workflowID string) (*WorkflowDefinition, error)
    UpdateWorkflowDefinition(ctx context.Context, workflowID string, workflow *WorkflowDefinition) error
    DeleteWorkflow(ctx context.Context, workflowID string) error
    ListWorkflows(ctx context.Context, filters WorkflowFilters) ([]*WorkflowDefinition, error)
    
    // Workflow state operations (future)
    CreateWorkflowState(ctx context.Context, workflowID string, state *WorkflowState) error
    GetWorkflowState(ctx context.Context, workflowID string) (*WorkflowState, error)
    UpdateWorkflowState(ctx context.Context, workflowID string, state *WorkflowState) error
    DeleteWorkflowState(ctx context.Context, workflowID string) error
    
    // Execution operations (future)
    CreateExecution(ctx context.Context, execution *WorkflowExecution) error
    GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error)
    UpdateExecution(ctx context.Context, executionID string, execution *WorkflowExecution) error
    ListExecutions(ctx context.Context, workflowID string, filters ExecutionFilters) ([]*WorkflowExecution, error)
    
    // Health and stats
    Health(ctx context.Context) error
    GetStats(ctx context.Context) (*DatabaseStats, error)
    Close() error
}
```

### WorkflowDefinition Model

Standardized workflow definition across all services:

```go
type WorkflowDefinition struct {
    ID          string                 `json:"id"`          // Workflow instance ID (wf_xxx)
    Name        string                 `json:"name"`        // User-provided name
    Description string                 `json:"description"` // Description
    Version     string                 `json:"version"`     // Version (e.g., "1.0.0")
    SpecVersion string                 `json:"spec_version"`// SDL spec version
    Definition  map[string]interface{} `json:"definition"`  // SDL definition
    CreatedAt   time.Time              `json:"created_at"`  // Creation timestamp
    UpdatedAt   time.Time              `json:"updated_at"`  // Update timestamp
    CreatedBy   string                 `json:"created_by"`  // Creator
    TenantID    string                 `json:"tenant_id"`   // Tenant isolation
}
```

## Storage Implementations

### Redis Implementation (`storage.RedisDatabase`)

Current implementation using Redis as the storage backend:

- **Key Pattern**: `workflow:definition:{workflowID}`
- **Serialization**: JSON
- **Tenant Isolation**: Included in workflow definition
- **Filtering**: In-memory filtering after retrieval

**Example Usage:**
```go
// Initialize Redis database
database := storage.NewRedisDatabase(redisClient, logger, &interfaces.DatabaseConfig{
    Type:       "redis",
    Host:       "localhost",
    Port:       6379,
    TenantMode: true,
})

// Create workflow
workflow := &interfaces.WorkflowDefinition{
    ID:          "wf_abc123",
    Name:        "My Workflow",
    TenantID:    "acme-inc",
    Definition:  workflowDSL,
}
err := database.CreateWorkflow(ctx, workflow)
```

### Future Implementations

- **PostgreSQL**: Relational database with proper indexing and complex queries
- **MongoDB**: Document database with flexible schema
- **DynamoDB**: AWS managed NoSQL database

## Service Integration

### API Service

Uses the shared database interface through an adapter:

```go
// Initialize shared database
database := storage.NewRedisDatabase(redisClient, logger, config)

// Create workflow service with shared database
workflowService := services.NewWorkflowService(database, executionRepo, eventStream, logger)
```

### Worker Service

Accesses the same shared database for workflow definitions:

```go
// Worker service reads workflow definitions from shared storage
workflow, err := database.GetWorkflowDefinition(ctx, workflowID)
```

### Event Streaming Integration

Stream keys are generated using workflow instance IDs from the shared storage:

- **Stream Key Pattern**: `tenant:{tenantID}:workflow:{workflowID}:events`
- **Tenant Isolation**: Each tenant gets separate streams
- **Workflow Isolation**: Each workflow instance gets its own stream

## Migration Strategy

### Phase 1: ✅ **Centralized Workflow Storage**
- Implement shared `Database` interface
- Create Redis implementation
- Migrate API service to use shared storage
- Eliminate duplicate workflow storage

### Phase 2: **Enhanced Filtering and Querying**
- Add advanced filtering capabilities
- Implement pagination and sorting
- Add full-text search support

### Phase 3: **Multi-Backend Support**
- Implement PostgreSQL backend
- Implement MongoDB backend
- Add configuration-based backend selection

### Phase 4: **State and Execution Storage**
- Implement workflow state storage
- Implement execution history storage
- Migrate from in-memory execution storage

## Configuration

### Database Configuration

```go
type DatabaseConfig struct {
    Type         string            `json:"type"`          // redis, postgresql, mongodb
    Host         string            `json:"host"`
    Port         int               `json:"port"`
    Database     string            `json:"database"`
    Username     string            `json:"username"`
    Password     string            `json:"password"`
    MaxConns     int               `json:"max_conns"`
    MaxIdleConns int               `json:"max_idle_conns"`
    Options      map[string]string `json:"options"`       // Backend-specific options
    TenantMode   bool              `json:"tenant_mode"`   // Enable multi-tenant isolation
}
```

### Environment Variables

```bash
# Database configuration
DATABASE_TYPE=redis
DATABASE_HOST=localhost
DATABASE_PORT=6379
DATABASE_TENANT_MODE=true

# Redis-specific
REDIS_PASSWORD=secret
REDIS_DB=0

# PostgreSQL-specific (future)
POSTGRES_DATABASE=flunq
POSTGRES_USERNAME=flunq
POSTGRES_PASSWORD=secret
POSTGRES_SSL_MODE=require
```

## Benefits

### 1. **Consistency**
- Single source of truth for all workflow data
- Consistent data model across all services
- No data synchronization issues

### 2. **Flexibility**
- Easy to swap storage backends
- Support for different storage requirements
- Future-proof architecture

### 3. **Tenant Isolation**
- Built-in multi-tenant support
- Secure data separation
- Scalable tenant management

### 4. **Performance**
- Optimized storage patterns
- Efficient querying and filtering
- Reduced data duplication

### 5. **Maintainability**
- Clean separation of concerns
- Testable storage layer
- Easy to extend and modify

## Best Practices

### 1. **Always Use Workflow Instance IDs**
- Use `wf_xxx` format for all workflow references
- Never use workflow definition names as keys
- Ensure unique identification across tenants

### 2. **Include Tenant Context**
- Always include `TenantID` in workflow definitions
- Filter by tenant in all queries
- Validate tenant access in all operations

### 3. **Handle Errors Gracefully**
- Check for "not found" errors
- Implement proper error handling
- Log storage operations for debugging

### 4. **Use Transactions When Available**
- Implement atomic operations where possible
- Handle concurrent access scenarios
- Ensure data consistency

## Monitoring and Observability

### Metrics
- Storage operation latency
- Error rates by operation type
- Storage utilization
- Tenant-specific metrics

### Logging
- All storage operations
- Error conditions
- Performance metrics
- Tenant isolation validation

### Health Checks
- Database connectivity
- Storage backend health
- Data consistency checks
- Performance benchmarks
