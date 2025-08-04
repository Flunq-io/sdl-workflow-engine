package adapters

import (
	"time"

	"go.uber.org/zap"

	"github.com/flunq-io/worker/internal/interfaces"
)

// ZapLogger adapts zap.Logger to the Logger interface
type ZapLogger struct {
	Logger *zap.Logger
}

// Debug logs a debug message
func (l *ZapLogger) Debug(msg string, fields ...interface{}) {
	l.Logger.Debug(msg, l.convertFields(fields...)...)
}

// Info logs an info message
func (l *ZapLogger) Info(msg string, fields ...interface{}) {
	l.Logger.Info(msg, l.convertFields(fields...)...)
}

// Warn logs a warning message
func (l *ZapLogger) Warn(msg string, fields ...interface{}) {
	l.Logger.Warn(msg, l.convertFields(fields...)...)
}

// Error logs an error message
func (l *ZapLogger) Error(msg string, fields ...interface{}) {
	l.Logger.Error(msg, l.convertFields(fields...)...)
}

// Fatal logs a fatal message and exits
func (l *ZapLogger) Fatal(msg string, fields ...interface{}) {
	l.Logger.Fatal(msg, l.convertFields(fields...)...)
}

// With returns a logger with additional fields
func (l *ZapLogger) With(fields ...interface{}) interfaces.Logger {
	return &ZapLogger{
		Logger: l.Logger.With(l.convertFields(fields...)...),
	}
}

// convertFields converts interface{} fields to zap.Field
func (l *ZapLogger) convertFields(fields ...interface{}) []zap.Field {
	if len(fields)%2 != 0 {
		// Odd number of fields, add the last one as a string
		fields = append(fields, "")
	}

	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}

		value := fields[i+1]
		zapFields = append(zapFields, l.convertValue(key, value))
	}

	return zapFields
}

// convertValue converts a value to the appropriate zap.Field
func (l *ZapLogger) convertValue(key string, value interface{}) zap.Field {
	switch v := value.(type) {
	case string:
		return zap.String(key, v)
	case int:
		return zap.Int(key, v)
	case int64:
		return zap.Int64(key, v)
	case float64:
		return zap.Float64(key, v)
	case bool:
		return zap.Bool(key, v)
	case time.Duration:
		return zap.Duration(key, v)
	case error:
		return zap.Error(v)
	case []string:
		return zap.Strings(key, v)
	default:
		return zap.Any(key, v)
	}
}

// NoOpMetrics is a no-op implementation of the Metrics interface
type NoOpMetrics struct{}

// IncrementCounter increments a counter metric
func (m *NoOpMetrics) IncrementCounter(name string, tags map[string]string) {
	// No-op
}

// RecordHistogram records a histogram metric
func (m *NoOpMetrics) RecordHistogram(name string, value float64, tags map[string]string) {
	// No-op
}

// RecordGauge records a gauge metric
func (m *NoOpMetrics) RecordGauge(name string, value float64, tags map[string]string) {
	// No-op
}

// StartTimer starts a timer and returns a function to stop it
func (m *NoOpMetrics) StartTimer(name string, tags map[string]string) func() {
	return func() {
		// No-op
	}
}
