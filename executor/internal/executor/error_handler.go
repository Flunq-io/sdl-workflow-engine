package executor

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DefaultErrorHandler implements the ErrorHandler interface
type DefaultErrorHandler struct {
	logger *zap.Logger
}

// NewErrorHandler creates a new DefaultErrorHandler
func NewErrorHandler(logger *zap.Logger) ErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError processes an error and determines the appropriate action
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error, config *CatchConfig, errorCtx *ErrorContext) (*ErrorHandlingResult, error) {
	h.logger.Debug("Handling error",
		zap.Error(err),
		zap.Int("attemptCount", errorCtx.AttemptCount))

	// Convert error to WorkflowError if needed
	workflowErr := h.toWorkflowError(err)
	errorCtx.Error = workflowErr

	// Check if error matches the filter
	if config.Errors != nil && !h.MatchesFilter(err, config.Errors) {
		h.logger.Debug("Error does not match filter, not catching")
		return &ErrorHandlingResult{Action: ErrorActionFail}, nil
	}

	// Evaluate conditional catch expressions
	if shouldCatch, err := h.evaluateCatchConditions(ctx, config, errorCtx); err != nil {
		return nil, fmt.Errorf("failed to evaluate catch conditions: %w", err)
	} else if !shouldCatch {
		h.logger.Debug("Catch conditions not met, not catching error")
		return &ErrorHandlingResult{Action: ErrorActionFail}, nil
	}

	// Prepare error data for runtime expressions
	errorData := map[string]interface{}{
		config.As: workflowErr,
	}
	if config.As == "" {
		errorData["error"] = workflowErr
	}

	// Check if we should retry
	if config.Retry != nil {
		shouldRetry, retryDelay, err := h.ShouldRetry(ctx, err, config.Retry, errorCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate retry policy: %w", err)
		}

		if shouldRetry {
			h.logger.Info("Retrying task after error",
				zap.Error(err),
				zap.Duration("delay", retryDelay),
				zap.Int("attempt", errorCtx.AttemptCount))

			return &ErrorHandlingResult{
				Action:     ErrorActionRetry,
				RetryDelay: retryDelay,
				ErrorData:  errorData,
			}, nil
		}
	}

	// Check if we have recovery tasks
	if config.Do != nil && len(config.Do) > 0 {
		h.logger.Info("Executing recovery tasks for error", zap.Error(err))
		return &ErrorHandlingResult{
			Action:        ErrorActionRecover,
			ErrorData:     errorData,
			RecoveryTasks: config.Do,
		}, nil
	}

	// Default action is to ignore the error (catch it without retrying or recovery)
	h.logger.Info("Catching error without retry or recovery", zap.Error(err))
	return &ErrorHandlingResult{
		Action:    ErrorActionIgnore,
		ErrorData: errorData,
	}, nil
}

// ShouldRetry determines if a task should be retried based on the retry policy
func (h *DefaultErrorHandler) ShouldRetry(ctx context.Context, err error, policy *RetryPolicy, errorCtx *ErrorContext) (bool, time.Duration, error) {
	// Check retry conditions
	if shouldRetry, err := h.evaluateRetryConditions(ctx, policy, errorCtx); err != nil {
		return false, 0, fmt.Errorf("failed to evaluate retry conditions: %w", err)
	} else if !shouldRetry {
		return false, 0, nil
	}

	// Check retry limits
	if h.exceedsRetryLimits(policy, errorCtx) {
		h.logger.Debug("Retry limits exceeded",
			zap.Int("attemptCount", errorCtx.AttemptCount),
			zap.Duration("totalDuration", errorCtx.TotalDuration))
		return false, 0, nil
	}

	// Calculate retry delay
	delay := h.calculateRetryDelay(policy, errorCtx)

	return true, delay, nil
}

// MatchesFilter checks if an error matches the specified filter
func (h *DefaultErrorHandler) MatchesFilter(err error, filter *ErrorFilter) bool {
	if filter == nil || filter.With == nil {
		return true // No filter means match all
	}

	workflowErr := h.toWorkflowError(err)

	for key, expectedValue := range filter.With {
		if !h.matchesProperty(workflowErr, key, expectedValue) {
			return false
		}
	}

	return true
}

// Helper methods

// toWorkflowError converts any error to a WorkflowError
func (h *DefaultErrorHandler) toWorkflowError(err error) *WorkflowError {
	if workflowErr, ok := err.(*WorkflowError); ok {
		return workflowErr
	}

	// Try to extract HTTP status from error message
	status := h.extractHTTPStatus(err.Error())

	// Determine error type based on status or error content
	errorType := h.determineErrorType(err, status)

	return &WorkflowError{
		Type:   errorType,
		Status: status,
		Title:  "Task Execution Error",
		Detail: err.Error(),
	}
}

// extractHTTPStatus attempts to extract HTTP status code from error message
func (h *DefaultErrorHandler) extractHTTPStatus(errMsg string) int {
	// Look for patterns like "status 503", "HTTP 404", etc.
	patterns := []string{
		`status (\d{3})`,
		`HTTP (\d{3})`,
		`(\d{3}) status`,
		`failed with status (\d{3})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
			if status, err := strconv.Atoi(matches[1]); err == nil {
				return status
			}
		}
	}

	return 500 // Default to internal server error
}

// determineErrorType determines the appropriate error type based on error content and status
func (h *DefaultErrorHandler) determineErrorType(err error, status int) string {
	errMsg := strings.ToLower(err.Error())

	// Check for specific error patterns
	switch {
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline"):
		return ErrorTypeTimeout
	case strings.Contains(errMsg, "unauthorized") || status == 401:
		return ErrorTypeAuthentication
	case strings.Contains(errMsg, "forbidden") || status == 403:
		return ErrorTypeAuthorization
	case strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "invalid"):
		return ErrorTypeValidation
	case strings.Contains(errMsg, "configuration") || strings.Contains(errMsg, "config"):
		return ErrorTypeConfiguration
	case strings.Contains(errMsg, "expression"):
		return ErrorTypeExpression
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "http") || status >= 500:
		return ErrorTypeCommunication
	default:
		return ErrorTypeRuntime
	}
}

// matchesProperty checks if a workflow error property matches the expected value
func (h *DefaultErrorHandler) matchesProperty(workflowErr *WorkflowError, key string, expectedValue interface{}) bool {
	var actualValue interface{}

	switch key {
	case "type":
		actualValue = workflowErr.Type
	case "status":
		actualValue = workflowErr.Status
	case "title":
		actualValue = workflowErr.Title
	case "detail":
		actualValue = workflowErr.Detail
	case "instance":
		actualValue = workflowErr.Instance
	default:
		// Check in additional data
		if workflowErr.Data != nil {
			actualValue = workflowErr.Data[key]
		}
	}

	return h.matchesValue(actualValue, expectedValue)
}

// matchesValue checks if an actual value matches an expected value (supports regex)
func (h *DefaultErrorHandler) matchesValue(actual, expected interface{}) bool {
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Direct equality check
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	// String regex matching
	actualStr, actualIsStr := actual.(string)
	expectedStr, expectedIsStr := expected.(string)

	if actualIsStr && expectedIsStr {
		// Try regex matching
		if matched, err := regexp.MatchString(expectedStr, actualStr); err == nil && matched {
			return true
		}
	}

	return false
}

// evaluateCatchConditions evaluates when/exceptWhen conditions for catching errors
func (h *DefaultErrorHandler) evaluateCatchConditions(ctx context.Context, config *CatchConfig, errorCtx *ErrorContext) (bool, error) {
	// If no conditions are specified, always catch
	if config.When == "" && config.ExceptWhen == "" {
		return true, nil
	}

	// TODO: Implement runtime expression evaluation
	// For now, we'll use simple string matching as a placeholder
	// In a real implementation, this would use a proper expression evaluator like jq

	if config.When != "" {
		// Placeholder: always return true for now
		// Real implementation would evaluate the runtime expression
		h.logger.Debug("Evaluating catch 'when' condition", zap.String("expression", config.When))
	}

	if config.ExceptWhen != "" {
		// Placeholder: always return false for now
		// Real implementation would evaluate the runtime expression and negate it
		h.logger.Debug("Evaluating catch 'exceptWhen' condition", zap.String("expression", config.ExceptWhen))
	}

	return true, nil
}

// evaluateRetryConditions evaluates when/exceptWhen conditions for retrying
func (h *DefaultErrorHandler) evaluateRetryConditions(ctx context.Context, policy *RetryPolicy, errorCtx *ErrorContext) (bool, error) {
	// If no conditions are specified, always retry (subject to limits)
	if policy.When == "" && policy.ExceptWhen == "" {
		return true, nil
	}

	// TODO: Implement runtime expression evaluation
	// For now, we'll use simple logic as a placeholder

	if policy.When != "" {
		h.logger.Debug("Evaluating retry 'when' condition", zap.String("expression", policy.When))
	}

	if policy.ExceptWhen != "" {
		h.logger.Debug("Evaluating retry 'exceptWhen' condition", zap.String("expression", policy.ExceptWhen))
	}

	return true, nil
}

// exceedsRetryLimits checks if retry limits have been exceeded
func (h *DefaultErrorHandler) exceedsRetryLimits(policy *RetryPolicy, errorCtx *ErrorContext) bool {
	if policy.Limit == nil {
		return false
	}

	// Check attempt count limit
	if policy.Limit.Attempt != nil && policy.Limit.Attempt.Count > 0 {
		if errorCtx.AttemptCount >= policy.Limit.Attempt.Count {
			return true
		}
	}

	// Check total duration limit
	if policy.Limit.Duration != nil {
		maxDuration := policy.Limit.Duration.ToDuration()
		if maxDuration > 0 && errorCtx.TotalDuration >= maxDuration {
			return true
		}
	}

	// Check attempt duration limit
	if policy.Limit.Attempt != nil && policy.Limit.Attempt.Duration != nil {
		maxAttemptDuration := policy.Limit.Attempt.Duration.ToDuration()
		if maxAttemptDuration > 0 && errorCtx.TotalDuration >= maxAttemptDuration {
			return true
		}
	}

	return false
}

// calculateRetryDelay calculates the delay before the next retry attempt
func (h *DefaultErrorHandler) calculateRetryDelay(policy *RetryPolicy, errorCtx *ErrorContext) time.Duration {
	baseDelay := time.Duration(0)

	// Start with the base delay
	if policy.Delay != nil {
		baseDelay = policy.Delay.ToDuration()
	}

	// Apply backoff strategy
	if policy.Backoff != nil {
		baseDelay = h.applyBackoffStrategy(baseDelay, policy.Backoff, errorCtx.AttemptCount)
	}

	// Apply jitter if configured
	if policy.Jitter != nil {
		baseDelay = h.applyJitter(baseDelay, policy.Jitter)
	}

	return baseDelay
}

// applyBackoffStrategy applies the configured backoff strategy
func (h *DefaultErrorHandler) applyBackoffStrategy(baseDelay time.Duration, strategy *BackoffStrategy, attemptCount int) time.Duration {
	switch {
	case strategy.Constant != nil:
		// Constant backoff - use base delay as-is
		return baseDelay

	case strategy.Exponential != nil:
		// Exponential backoff
		multiplier := strategy.Exponential.Multiplier
		if multiplier <= 0 {
			multiplier = 2.0 // Default multiplier
		}

		// Calculate exponential delay: baseDelay * multiplier^(attemptCount-1)
		delay := float64(baseDelay)
		for i := 1; i < attemptCount; i++ {
			delay *= multiplier
		}

		result := time.Duration(delay)

		// Apply max delay cap if configured
		if strategy.Exponential.MaxDelay != nil {
			maxDelay := strategy.Exponential.MaxDelay.ToDuration()
			if maxDelay > 0 && result > maxDelay {
				result = maxDelay
			}
		}

		return result

	case strategy.Linear != nil:
		// Linear backoff
		increment := time.Duration(0)
		if strategy.Linear.Increment != nil {
			increment = strategy.Linear.Increment.ToDuration()
		}

		// Calculate linear delay: baseDelay + (increment * (attemptCount-1))
		return baseDelay + time.Duration(attemptCount-1)*increment

	default:
		// No backoff strategy specified, use base delay
		return baseDelay
	}
}

// applyJitter applies random jitter to the delay
func (h *DefaultErrorHandler) applyJitter(delay time.Duration, jitter *JitterConfig) time.Duration {
	if jitter.From == nil || jitter.To == nil {
		return delay
	}

	minJitter := jitter.From.ToDuration()
	maxJitter := jitter.To.ToDuration()

	if minJitter >= maxJitter {
		return delay + minJitter
	}

	// Generate random jitter between min and max
	jitterRange := maxJitter - minJitter
	randomJitter := time.Duration(rand.Int63n(int64(jitterRange))) + minJitter

	return delay + randomJitter
}
