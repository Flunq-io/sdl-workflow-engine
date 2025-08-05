package interfaces

import (
	"context"
	"time"

	"github.com/flunq-io/shared/pkg/cloudevents"
)

// EventStorage defines the generic interface for event persistence
// Can be implemented by Redis, PostgreSQL, MongoDB, EventStore DB, etc.
type EventStorage interface {
	// Store persists an event
	Store(ctx context.Context, event *cloudevents.CloudEvent) error

	// GetEventHistory retrieves all events for a workflow (with optional tenant isolation)
	GetEventHistory(ctx context.Context, workflowID string) ([]*cloudevents.CloudEvent, error)

	// GetEventHistoryForTenant retrieves all events for a workflow within a specific tenant
	GetEventHistoryForTenant(ctx context.Context, tenantID, workflowID string) ([]*cloudevents.CloudEvent, error)

	// GetExecutionHistory retrieves all events for a specific execution
	GetExecutionHistory(ctx context.Context, tenantID, executionID string) ([]*cloudevents.CloudEvent, error)

	// GetEventsSince retrieves events since a specific version/timestamp
	GetEventsSince(ctx context.Context, workflowID string, since string) ([]*cloudevents.CloudEvent, error)

	// GetEventsByType retrieves events by type
	GetEventsByType(ctx context.Context, eventType string, limit int) ([]*cloudevents.CloudEvent, error)

	// GetEventsByTimeRange retrieves events within a time range
	GetEventsByTimeRange(ctx context.Context, start, end time.Time) ([]*cloudevents.CloudEvent, error)

	// DeleteEvents deletes events for a workflow
	DeleteEvents(ctx context.Context, workflowID string) error

	// GetStats returns storage statistics
	GetStats(ctx context.Context) (*StorageStats, error)

	// Health checks storage health
	Health(ctx context.Context) error

	// Close closes the storage connection
	Close() error
}

// StorageStats represents storage statistics
type StorageStats struct {
	TotalEvents     int64     `json:"total_events"`
	TotalWorkflows  int64     `json:"total_workflows"`
	TotalExecutions int64     `json:"total_executions"`
	StorageSize     int64     `json:"storage_size_bytes"`
	LastEventTime   time.Time `json:"last_event_time"`
	Health          string    `json:"health"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	Type       string            `json:"type"`        // redis, postgresql, mongodb, etc.
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Database   string            `json:"database"`
	Username   string            `json:"username"`
	Password   string            `json:"password"`
	Options    map[string]string `json:"options"`     // Additional storage-specific options
	TenantMode bool              `json:"tenant_mode"` // Enable multi-tenant isolation
}
