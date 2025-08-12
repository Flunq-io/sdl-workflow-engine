package executor

import (
	"context"
	"time"
)

// TryTaskConfig represents the configuration for a try task
type TryTaskConfig struct {
	Try   map[string]*TaskRequest `json:"try" yaml:"try"`
	Catch *CatchConfig            `json:"catch" yaml:"catch"`
}

// CatchConfig defines how to catch and handle errors
type CatchConfig struct {
	Errors     *ErrorFilter            `json:"errors,omitempty" yaml:"errors,omitempty"`
	As         string                  `json:"as,omitempty" yaml:"as,omitempty"`                 // Variable name to store error (defaults to "error")
	When       string                  `json:"when,omitempty" yaml:"when,omitempty"`             // Runtime expression to determine if should catch
	ExceptWhen string                  `json:"exceptWhen,omitempty" yaml:"exceptWhen,omitempty"` // Runtime expression to determine if should NOT catch
	Retry      *RetryPolicy            `json:"retry,omitempty" yaml:"retry,omitempty"`           // Retry policy to apply
	Do         map[string]*TaskRequest `json:"do,omitempty" yaml:"do,omitempty"`                 // Tasks to run when catching error
}

// ErrorFilter defines criteria for matching errors
type ErrorFilter struct {
	With map[string]interface{} `json:"with" yaml:"with"` // Error properties to match (type, status, etc.)
}

// RetryPolicy defines the strategy for retrying failed tasks
type RetryPolicy struct {
	When       string           `json:"when,omitempty" yaml:"when,omitempty"`             // Runtime expression to determine if should retry
	ExceptWhen string           `json:"exceptWhen,omitempty" yaml:"exceptWhen,omitempty"` // Runtime expression to determine if should NOT retry
	Limit      *RetryLimit      `json:"limit,omitempty" yaml:"limit,omitempty"`           // Retry limits
	Backoff    *BackoffStrategy `json:"backoff,omitempty" yaml:"backoff,omitempty"`       // Backoff strategy
	Jitter     *JitterConfig    `json:"jitter,omitempty" yaml:"jitter,omitempty"`         // Jitter configuration
	Delay      *Duration        `json:"delay,omitempty" yaml:"delay,omitempty"`           // Initial delay before first retry
}

// RetryLimit defines limits for retry attempts
type RetryLimit struct {
	Attempt  *AttemptLimit `json:"attempt,omitempty" yaml:"attempt,omitempty"`   // Attempt-based limits
	Duration *Duration     `json:"duration,omitempty" yaml:"duration,omitempty"` // Time-based limit for all retries
}

// AttemptLimit defines attempt-based retry limits
type AttemptLimit struct {
	Count    int       `json:"count,omitempty" yaml:"count,omitempty"`       // Maximum number of attempts
	Duration *Duration `json:"duration,omitempty" yaml:"duration,omitempty"` // Duration limit for all attempts
}

// BackoffStrategy defines the backoff strategy for retries
type BackoffStrategy struct {
	Constant    *ConstantBackoff    `json:"constant,omitempty" yaml:"constant,omitempty"`
	Exponential *ExponentialBackoff `json:"exponential,omitempty" yaml:"exponential,omitempty"`
	Linear      *LinearBackoff      `json:"linear,omitempty" yaml:"linear,omitempty"`
}

// ConstantBackoff defines constant delay backoff
type ConstantBackoff struct {
	// Empty struct - uses the delay from RetryPolicy
}

// ExponentialBackoff defines exponential backoff strategy
type ExponentialBackoff struct {
	Multiplier float64   `json:"multiplier,omitempty" yaml:"multiplier,omitempty"` // Multiplier for each retry (defaults to 2.0)
	MaxDelay   *Duration `json:"maxDelay,omitempty" yaml:"maxDelay,omitempty"`     // Maximum delay cap
}

// LinearBackoff defines linear backoff strategy
type LinearBackoff struct {
	Increment *Duration `json:"increment,omitempty" yaml:"increment,omitempty"` // Amount to add each retry
}

// JitterConfig defines jitter parameters for retry delays
type JitterConfig struct {
	From *Duration `json:"from" yaml:"from"` // Minimum jitter duration
	To   *Duration `json:"to" yaml:"to"`     // Maximum jitter duration
}

// Duration represents a time duration with multiple units
type Duration struct {
	Days         int `json:"days,omitempty" yaml:"days,omitempty"`
	Hours        int `json:"hours,omitempty" yaml:"hours,omitempty"`
	Minutes      int `json:"minutes,omitempty" yaml:"minutes,omitempty"`
	Seconds      int `json:"seconds,omitempty" yaml:"seconds,omitempty"`
	Milliseconds int `json:"milliseconds,omitempty" yaml:"milliseconds,omitempty"`
}

// ToDuration converts Duration to time.Duration
func (d *Duration) ToDuration() time.Duration {
	if d == nil {
		return 0
	}

	duration := time.Duration(d.Days) * 24 * time.Hour
	duration += time.Duration(d.Hours) * time.Hour
	duration += time.Duration(d.Minutes) * time.Minute
	duration += time.Duration(d.Seconds) * time.Second
	duration += time.Duration(d.Milliseconds) * time.Millisecond

	return duration
}

// WorkflowError represents a structured error following Problem Details RFC
type WorkflowError struct {
	Type     string                 `json:"type"`               // Error type URI
	Status   int                    `json:"status"`             // HTTP status code
	Instance string                 `json:"instance,omitempty"` // JSON Pointer to error source
	Title    string                 `json:"title,omitempty"`    // Human-readable summary
	Detail   string                 `json:"detail,omitempty"`   // Human-readable explanation
	Data     map[string]interface{} `json:"data,omitempty"`     // Additional error data
}

// Error implements the error interface
func (e *WorkflowError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	if e.Title != "" {
		return e.Title
	}
	return e.Type
}

// ErrorContext provides context for error handling and retry decisions
type ErrorContext struct {
	Error         *WorkflowError         `json:"error"`
	AttemptCount  int                    `json:"attemptCount"`
	TotalDuration time.Duration          `json:"totalDuration"`
	LastAttempt   time.Time              `json:"lastAttempt"`
	TaskInput     map[string]interface{} `json:"taskInput"`
	WorkflowData  map[string]interface{} `json:"workflowData"`
}

// ErrorHandler defines the interface for handling errors
type ErrorHandler interface {
	// HandleError processes an error and determines the appropriate action
	HandleError(ctx context.Context, err error, config *CatchConfig, errorCtx *ErrorContext) (*ErrorHandlingResult, error)

	// ShouldRetry determines if a task should be retried based on the retry policy
	ShouldRetry(ctx context.Context, err error, policy *RetryPolicy, errorCtx *ErrorContext) (bool, time.Duration, error)

	// MatchesFilter checks if an error matches the specified filter
	MatchesFilter(err error, filter *ErrorFilter) bool
}

// ErrorHandlingResult represents the result of error handling
type ErrorHandlingResult struct {
	Action        ErrorAction             `json:"action"`
	RetryDelay    time.Duration           `json:"retryDelay,omitempty"`
	ErrorData     map[string]interface{}  `json:"errorData,omitempty"`
	RecoveryTasks map[string]*TaskRequest `json:"recoveryTasks,omitempty"`
}

// ErrorAction defines the action to take after error handling
type ErrorAction int

const (
	ErrorActionRetry ErrorAction = iota
	ErrorActionRecover
	ErrorActionFail
	ErrorActionIgnore
)

// Standard error types from SDL specification
const (
	ErrorTypeConfiguration  = "https://serverlessworkflow.io/spec/1.0.0/errors/configuration"
	ErrorTypeValidation     = "https://serverlessworkflow.io/spec/1.0.0/errors/validation"
	ErrorTypeExpression     = "https://serverlessworkflow.io/spec/1.0.0/errors/expression"
	ErrorTypeAuthentication = "https://serverlessworkflow.io/spec/1.0.0/errors/authentication"
	ErrorTypeAuthorization  = "https://serverlessworkflow.io/spec/1.0.0/errors/authorization"
	ErrorTypeTimeout        = "https://serverlessworkflow.io/spec/1.0.0/errors/timeout"
	ErrorTypeCommunication  = "https://serverlessworkflow.io/spec/1.0.0/errors/communication"
	ErrorTypeRuntime        = "https://serverlessworkflow.io/spec/1.0.0/errors/runtime"
)
