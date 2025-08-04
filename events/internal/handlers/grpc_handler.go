package handlers

import (
	"go.uber.org/zap"

	"github.com/flunq-io/events/internal/subscriber"
	"github.com/flunq-io/events/pkg/cloudevents"
)

// GRPCHandler handles gRPC requests for the Event Store
type GRPCHandler struct {
	cloudEventsHandler *cloudevents.Handler
	subscriberManager  *subscriber.Manager
	logger             *zap.Logger
}

// NewGRPCHandler creates a new gRPC handler
func NewGRPCHandler(
	cloudEventsHandler *cloudevents.Handler,
	subscriberManager *subscriber.Manager,
	logger *zap.Logger,
) *GRPCHandler {
	return &GRPCHandler{
		cloudEventsHandler: cloudEventsHandler,
		subscriberManager:  subscriberManager,
		logger:             logger,
	}
}

// TODO: Implement gRPC methods based on proto definitions
// This is a placeholder for the gRPC service implementation
