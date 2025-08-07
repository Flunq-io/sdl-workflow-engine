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
	"google.golang.org/grpc"

	"github.com/flunq-io/events/internal/core"
	"github.com/flunq-io/events/internal/handlers"
	"github.com/flunq-io/events/internal/middleware"
	"github.com/flunq-io/events/internal/publisher"
	"github.com/flunq-io/events/internal/storage"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	config := loadConfig()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPassword,
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize storage and publisher using interfaces
	storage := storage.NewRedisStorage(redisClient, logger)
	publisher := publisher.NewRedisPublisher(redisClient, logger)

	// Initialize core event store
	eventStore := core.NewEventStore(storage, publisher, logger)

	logger.Info("Event Store service starting",
		zap.Bool("http_enabled", config.EnableHTTP),
		zap.Bool("grpc_enabled", config.EnableGRPC),
		zap.String("mode", config.Mode))

	var httpServer *http.Server
	var grpcServer *grpc.Server

	// Optionally start HTTP server
	if config.EnableHTTP {
		httpHandler := handlers.NewSimpleHTTPHandler(eventStore, logger)
		router := setupHTTPRouter(httpHandler)

		httpServer = &http.Server{
			Addr:    ":" + config.Port,
			Handler: router,
		}

		go func() {
			logger.Info("Starting HTTP server", zap.String("port", config.Port))
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatal("Failed to start HTTP server", zap.Error(err))
			}
		}()
	}

	// Optionally start gRPC server (disabled for now)
	if config.EnableGRPC {
		logger.Info("gRPC server disabled - not yet implemented with new interfaces")
		// TODO: Implement gRPC handler with new interfaces
	}

	// If neither HTTP nor gRPC is enabled, run in pure pub/sub mode
	if !config.EnableHTTP && !config.EnableGRPC {
		logger.Info("Running in pure pub/sub mode - no HTTP/gRPC servers")
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Event Store service...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop servers if they were started
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Fatal("Server forced to shutdown", zap.Error(err))
		}
	}

	// Close event store
	eventStore.Close()

	logger.Info("Event Store service exited")
}

func setupHTTPRouter(
	httpHandler *handlers.SimpleHTTPHandler,
) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())

	// Health endpoints
	router.GET("/health", httpHandler.Health)
	router.GET("/metrics", httpHandler.Metrics)

	// Event API endpoints
	v1 := router.Group("/api/v1")
	{
		// Event operations (workflow-based - backward compatibility)
		events := v1.Group("/events")
		{
			events.POST("", httpHandler.PublishEvent)
			events.GET("/:workflowId", httpHandler.GetEventHistory)
			events.GET("/:workflowId/since/:version", httpHandler.GetEventsSince)
		}

		// Execution-based event operations (new)
		executions := v1.Group("/executions")
		{
			executions.GET("/:executionId/events", httpHandler.GetExecutionHistory)
		}

		// Workflow execution management (new)
		workflows := v1.Group("/workflows")
		{
			workflows.GET("/:workflowId/executions", httpHandler.GetWorkflowExecutions)
		}

		// Subscription management
		v1.POST("/subscribe", httpHandler.Subscribe)

		// Event Store management
		admin := v1.Group("/admin")
		{
			admin.GET("/stats", httpHandler.GetStats)
			admin.POST("/replay/:workflowId", httpHandler.ReplayEvents)
		}
	}

	return router
}

type Config struct {
	Port          string
	GRPCPort      string
	RedisURL      string
	RedisPassword string
	LogLevel      string
	EnableHTTP    bool
	EnableGRPC    bool
	Mode          string
}

func loadConfig() *Config {
	return &Config{
		Port:          getEnv("PORT", "8081"),
		GRPCPort:      getEnv("GRPC_PORT", "9001"),
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		EnableHTTP:    getEnvBool("ENABLE_HTTP", true),
		EnableGRPC:    getEnvBool("ENABLE_GRPC", false),
		Mode:          getEnv("MODE", "full"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
