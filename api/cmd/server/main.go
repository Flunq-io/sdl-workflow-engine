package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/adapters"
	"github.com/flunq-io/api/internal/handlers"
	"github.com/flunq-io/api/internal/middleware"
	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/api/internal/services"
	"github.com/flunq-io/shared/pkg/factory"
	"github.com/flunq-io/shared/pkg/interfaces"
)

// LoggerAdapter adapts zap.Logger to eventstore.Logger interface
type LoggerAdapter struct {
	logger *zap.Logger
}

func (l *LoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, convertToZapFields(keysAndValues...)...)
}

func (l *LoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, convertToZapFields(keysAndValues...)...)
}

func (l *LoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, convertToZapFields(keysAndValues...)...)
}

func (l *LoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, convertToZapFields(keysAndValues...)...)
}

// DatabaseLoggerAdapter adapts zap.Logger to interfaces.Logger interface
type DatabaseLoggerAdapter struct {
	logger *zap.Logger
}

func (l *DatabaseLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, convertToZapFields(keysAndValues...)...)
}

func (l *DatabaseLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, convertToZapFields(keysAndValues...)...)
}

func (l *DatabaseLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, convertToZapFields(keysAndValues...)...)
}

func (l *DatabaseLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, convertToZapFields(keysAndValues...)...)
}

func (l *DatabaseLoggerAdapter) Fatal(msg string, keysAndValues ...interface{}) {
	l.logger.Fatal(msg, convertToZapFields(keysAndValues...)...)
}

func (l *DatabaseLoggerAdapter) With(keysAndValues ...interface{}) interfaces.Logger {
	return &DatabaseLoggerAdapter{logger: l.logger.With(convertToZapFields(keysAndValues...)...)}
}

// WorkflowRepositoryAdapter adapts Database interface to WorkflowRepository interface
// TODO: Remove this when ExecutionService is updated to use shared interfaces
type WorkflowRepositoryAdapter struct {
	database interfaces.Database
}

func (w *WorkflowRepositoryAdapter) Create(ctx context.Context, workflow *models.Workflow) error {
	// Convert to shared WorkflowDefinition
	sharedWorkflow := &interfaces.WorkflowDefinition{
		ID:          workflow.ID,
		Name:        workflow.Name,
		Description: workflow.Description,
		Version:     "1.0.0",
		SpecVersion: "1.0.0",
		Definition:  workflow.Definition,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
		CreatedBy:   "api",
		TenantID:    workflow.TenantID,
	}
	return w.database.CreateWorkflow(ctx, sharedWorkflow)
}

func (w *WorkflowRepositoryAdapter) GetByID(ctx context.Context, id string) (*models.WorkflowDetail, error) {
	sharedWorkflow, err := w.database.GetWorkflowDefinition(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert back to API model
	workflow := &models.Workflow{
		ID:          sharedWorkflow.ID,
		Name:        sharedWorkflow.Name,
		Description: sharedWorkflow.Description,
		TenantID:    sharedWorkflow.TenantID,
		Definition:  sharedWorkflow.Definition,
		State:       models.WorkflowStateActive,
		Tags:        []string{},
		CreatedAt:   sharedWorkflow.CreatedAt,
		UpdatedAt:   sharedWorkflow.UpdatedAt,
	}

	return &models.WorkflowDetail{
		Workflow:       *workflow,
		ExecutionCount: 0,
		LastExecution:  nil,
		Input:          nil,
		Output:         nil,
		TaskExecutions: nil,
	}, nil
}

func (w *WorkflowRepositoryAdapter) Update(ctx context.Context, workflow *models.Workflow) error {
	sharedWorkflow := &interfaces.WorkflowDefinition{
		ID:          workflow.ID,
		Name:        workflow.Name,
		Description: workflow.Description,
		Version:     "1.0.0",
		SpecVersion: "1.0.0",
		Definition:  workflow.Definition,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
		CreatedBy:   "api",
		TenantID:    workflow.TenantID,
	}
	return w.database.UpdateWorkflowDefinition(ctx, workflow.ID, sharedWorkflow)
}

func (w *WorkflowRepositoryAdapter) Delete(ctx context.Context, id string) error {
	return w.database.DeleteWorkflow(ctx, id)
}

func (w *WorkflowRepositoryAdapter) List(ctx context.Context, params *models.WorkflowListParams) ([]models.Workflow, int, error) {
	filters := interfaces.WorkflowFilters{
		Limit:  params.Limit,
		Offset: params.Offset,
		// TODO: Add TenantID and Name filters when available in WorkflowListParams
	}

	sharedWorkflows, err := w.database.ListWorkflows(ctx, filters)
	if err != nil {
		return nil, 0, err
	}

	workflows := make([]models.Workflow, len(sharedWorkflows))
	for i, sharedWorkflow := range sharedWorkflows {
		workflows[i] = models.Workflow{
			ID:          sharedWorkflow.ID,
			Name:        sharedWorkflow.Name,
			Description: sharedWorkflow.Description,
			TenantID:    sharedWorkflow.TenantID,
			Definition:  sharedWorkflow.Definition,
			State:       models.WorkflowStateActive,
			Tags:        []string{},
			CreatedAt:   sharedWorkflow.CreatedAt,
			UpdatedAt:   sharedWorkflow.UpdatedAt,
		}
	}

	return workflows, len(workflows), nil
}

func convertToZapFields(keysAndValues ...interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := keysAndValues[i].(string)
			value := keysAndValues[i+1]
			fields = append(fields, zap.Any(key, value))
		}
	}
	return fields
}

// EventStreamLoggerAdapter adapts zap.Logger to the shared eventstreaming.Logger interface
type EventStreamLoggerAdapter struct {
	logger *zap.Logger
}

func (l *EventStreamLoggerAdapter) Debug(msg string, fields ...interface{}) {
	zapFields := l.convertFields(fields...)
	l.logger.Debug(msg, zapFields...)
}

func (l *EventStreamLoggerAdapter) Info(msg string, fields ...interface{}) {
	zapFields := l.convertFields(fields...)
	l.logger.Info(msg, zapFields...)
}

func (l *EventStreamLoggerAdapter) Error(msg string, fields ...interface{}) {
	zapFields := l.convertFields(fields...)
	l.logger.Error(msg, zapFields...)
}

func (l *EventStreamLoggerAdapter) Warn(msg string, fields ...interface{}) {
	zapFields := l.convertFields(fields...)
	l.logger.Warn(msg, zapFields...)
}

func (l *EventStreamLoggerAdapter) convertFields(fields ...interface{}) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_URL", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Select EventStream implementation by config via shared factory
	eventBackend := getEnv("EVENT_STREAM_TYPE", "redis")
	eventStream, err := factory.NewEventStream(factory.EventStreamDeps{
		Backend:     eventBackend,
		RedisClient: redisClient,
		Logger:      &EventStreamLoggerAdapter{logger: logger},
	})
	if err != nil {
		logger.Fatal("Failed to create EventStream", zap.Error(err))
	}

	// Select Database implementation by config via shared factory
	dbBackend := getEnv("DB_TYPE", "redis")
	database, err := factory.NewDatabase(factory.DatabaseDeps{
		Backend:     dbBackend,
		RedisClient: redisClient,
		Logger:      &DatabaseLoggerAdapter{logger: logger},
		Config: &interfaces.DatabaseConfig{
			Type:       dbBackend,
			Host:       getEnv("REDIS_HOST", "localhost"),
			Port:       6379,
			TenantMode: true,
		},
	})
	if err != nil {
		logger.Fatal("Failed to create Database", zap.Error(err))
	}

	// eventStream and database already selected above via config

	// Initialize execution repository using shared database
	executionRepo := adapters.NewSharedExecutionRepository(database)

	// Initialize services
	// Cast to services.ExecutionRepository interface
	var servicesExecutionRepo services.ExecutionRepository = executionRepo
	workflowService := services.NewWorkflowService(database, servicesExecutionRepo, eventStream, redisClient, logger)
	// Create workflow adapter for execution service
	workflowAdapter := &WorkflowRepositoryAdapter{database: database}
	executionService := services.NewExecutionService(executionRepo, workflowAdapter, eventStream, redisClient, logger)

	// Initialize handlers
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logger)
	executionHandler := handlers.NewExecutionHandler(executionService, logger)
	healthHandler := handlers.NewHealthHandler(redisClient, logger)
	docsHandler := handlers.NewDocsHandler()

	// Setup router
	router := setupRouter(workflowHandler, executionHandler, healthHandler, docsHandler)

	// Start server
	port := getEnv("PORT", "8080")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func setupRouter(
	workflowHandler *handlers.WorkflowHandler,
	executionHandler *handlers.ExecutionHandler,
	healthHandler *handlers.HealthHandler,
	docsHandler *handlers.DocsHandler,
) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())

	// Health endpoints
	router.GET("/health", healthHandler.Health)
	router.GET("/metrics", healthHandler.Metrics)

	// API v1 routes with tenant context
	v1 := router.Group("/api/v1/:tenant_id")
	{
		// Workflow routes
		workflows := v1.Group("/workflows")
		{
			workflows.POST("", workflowHandler.Create)
			workflows.GET("", workflowHandler.List)
			workflows.GET("/:id", workflowHandler.Get)
			workflows.PUT("/:id", workflowHandler.Update)
			workflows.DELETE("/:id", workflowHandler.Delete)
			workflows.POST("/:id/execute", workflowHandler.Execute)
			workflows.GET("/:id/events", workflowHandler.GetEvents)
			workflows.GET("/:id/executions", workflowHandler.GetExecutions)
		}

		// Execution routes
		executions := v1.Group("/executions")
		{
			executions.GET("", executionHandler.List)
			executions.GET("/:id", executionHandler.Get)
			executions.GET("/:id/events", executionHandler.GetEvents)
			executions.POST("/:id/cancel", executionHandler.Cancel)
		}
	}

	// Documentation routes (outside tenant scope)
	apiV1 := router.Group("/api/v1")
	docs := apiV1.Group("/docs")
	{
		docs.GET("/openapi.yaml", docsHandler.OpenAPISpec)
	}

	// Swagger UI (outside of /api/v1)
	router.GET("/docs", docsHandler.SwaggerUI)
	router.GET("/docs/", docsHandler.SwaggerUI)

	return router
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
