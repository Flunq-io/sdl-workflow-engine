package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	redisClient *redis.Client
	logger      *zap.Logger
	version     string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(redisClient *redis.Client, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		redisClient: redisClient,
		logger:      logger,
		version:     "1.0.0", // This could be injected from build info
	}
}

// Health handles GET /health
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	dependencies := make(map[string]string)
	overallStatus := "healthy"

	// Check Redis connectivity
	redisStatus := h.checkRedis(ctx)
	dependencies["redis"] = redisStatus
	if redisStatus != "healthy" {
		overallStatus = "unhealthy"
	}

	// Check Event Store connectivity
	eventStoreStatus := h.checkEventStore(ctx)
	dependencies["event_store"] = eventStoreStatus
	if eventStoreStatus != "healthy" {
		overallStatus = "unhealthy"
	}

	response := models.NewHealthResponse(overallStatus, h.version, dependencies)

	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// Metrics handles GET /metrics
func (h *HealthHandler) Metrics(c *gin.Context) {
	// TODO: Implement Prometheus metrics
	// For now, return basic metrics
	metrics := map[string]interface{}{
		"service":   "flunq-api",
		"version":   h.version,
		"timestamp": time.Now(),
		"uptime":    "not_implemented",
		"requests":  "not_implemented",
		"errors":    "not_implemented",
	}

	c.JSON(http.StatusOK, metrics)
}

// checkRedis checks Redis connectivity
func (h *HealthHandler) checkRedis(ctx context.Context) string {
	if h.redisClient == nil {
		return "unhealthy"
	}

	err := h.redisClient.Ping(ctx).Err()
	if err != nil {
		h.logger.Error("Redis health check failed", zap.Error(err))
		return "unhealthy"
	}

	return "healthy"
}

// checkEventStore checks Event Store connectivity
func (h *HealthHandler) checkEventStore(ctx context.Context) string {
	// TODO: Implement actual Event Store health check
	// For now, assume it's healthy if we can reach it
	// This could be enhanced to make an actual HTTP request to the Event Store health endpoint
	
	// Placeholder implementation
	return "healthy"
}
