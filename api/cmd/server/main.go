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

	"github.com/flunq-io/api/internal/handlers"
	"github.com/flunq-io/api/internal/middleware"
	"github.com/flunq-io/api/internal/repository"
	"github.com/flunq-io/api/internal/services"
	"github.com/flunq-io/events/pkg/client"
)

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

	// Initialize Event Store client
	eventStoreURL := getEnv("EVENTS_URL", "http://localhost:8081")
	eventClient := client.NewEventClient(eventStoreURL, "api-service", logger)

	// Initialize repositories
	workflowRepo := repository.NewRedisWorkflowRepository(redisClient, logger)
	executionRepo := repository.NewMemoryExecutionRepository()

	// Initialize services
	workflowService := services.NewWorkflowService(workflowRepo, executionRepo, eventClient, logger)
	executionService := services.NewExecutionService(executionRepo, eventClient, logger)

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

	// API v1 routes
	v1 := router.Group("/api/v1")
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
		}

		// Execution routes
		executions := v1.Group("/executions")
		{
			executions.GET("", executionHandler.List)
			executions.GET("/:id", executionHandler.Get)
			executions.POST("/:id/cancel", executionHandler.Cancel)
		}

		// Documentation routes
		docs := v1.Group("/docs")
		{
			docs.GET("/openapi.yaml", docsHandler.OpenAPISpec)
		}
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
