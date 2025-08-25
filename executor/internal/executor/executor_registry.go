package executor

import (
	sharedinterfaces "github.com/flunq-io/shared/pkg/interfaces"
	"go.uber.org/zap"
)

// ExecutorRegistry manages all task executors
type ExecutorRegistry struct {
	executors   map[string]TaskExecutor
	logger      *zap.Logger
	eventStream sharedinterfaces.EventStream
}

// NewExecutorRegistry creates a new executor registry with all SDL-compliant executors
func NewExecutorRegistry(logger *zap.Logger, eventStream sharedinterfaces.EventStream) *ExecutorRegistry {
	registry := &ExecutorRegistry{
		executors:   make(map[string]TaskExecutor),
		logger:      logger,
		eventStream: eventStream,
	}

	// Register all task executors
	registry.registerExecutors()

	return registry
}

// registerExecutors registers all available task executors
func (r *ExecutorRegistry) registerExecutors() {
	// Create executor map for try task executor
	executorMap := make(map[string]TaskExecutor)

	// Register basic executors
	callExecutor := NewCallTaskExecutor(r.logger)
	setExecutor := NewSetTaskExecutor(r.logger)
	waitExecutor := NewWaitTaskExecutor(r.logger)
	injectExecutor := NewInjectTaskExecutor(r.logger)

	r.executors["call"] = callExecutor
	r.executors["set"] = setExecutor
	r.executors["wait"] = waitExecutor
	r.executors["inject"] = injectExecutor

	// Add to executor map for try task executor
	executorMap["call"] = callExecutor
	executorMap["set"] = setExecutor
	executorMap["wait"] = waitExecutor
	executorMap["inject"] = injectExecutor

	// Register try task executor for SDL try/catch handling
	tryExecutor := NewTryTaskExecutor(r.logger, executorMap, r.eventStream)
	r.executors["try"] = tryExecutor

	r.logger.Info("Registered task executors",
		zap.Int("count", len(r.executors)),
		zap.Strings("types", r.getRegisteredTypes()))
}

// GetExecutor returns the executor for the specified task type
func (r *ExecutorRegistry) GetExecutor(taskType string) (TaskExecutor, bool) {
	r.logger.Info("🔍 EXECUTOR REGISTRY DEBUG",
		zap.String("requested_task_type", taskType),
		zap.Strings("available_types", r.getRegisteredTypes()))

	executor, exists := r.executors[taskType]
	if !exists {
		r.logger.Error("❌ NO EXECUTOR FOUND",
			zap.String("requested_task_type", taskType))
	} else {
		r.logger.Info("✅ EXECUTOR FOUND",
			zap.String("requested_task_type", taskType),
			zap.String("executor_type", executor.GetTaskType()))
	}
	return executor, exists
}

// GetAllExecutors returns a map of all registered executors
func (r *ExecutorRegistry) GetAllExecutors() map[string]TaskExecutor {
	// Return a copy to prevent external modification
	result := make(map[string]TaskExecutor)
	for k, v := range r.executors {
		result[k] = v
	}
	return result
}

// RegisterExecutor allows registering custom executors
func (r *ExecutorRegistry) RegisterExecutor(taskType string, executor TaskExecutor) {
	r.executors[taskType] = executor
	r.logger.Info("Registered custom executor", zap.String("taskType", taskType))
}

// getRegisteredTypes returns a list of all registered task types
func (r *ExecutorRegistry) getRegisteredTypes() []string {
	types := make([]string, 0, len(r.executors))
	for taskType := range r.executors {
		types = append(types, taskType)
	}
	return types
}
