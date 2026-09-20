// Package logger provides structured JSON logging with correlation ID extraction.
package logger

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// CorrelationIDKey is the context key for correlation IDs.
	CorrelationIDKey contextKey = "correlation_id"
)

var (
	// globalLogger is the global logger instance.
	globalLogger *zap.Logger
)

// Initialize sets up the global logger with the specified log level.
func Initialize(logLevel string) error {
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	config := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if os.Getenv("ENVIRONMENT") == "production" {
		encoder = zapcore.NewJSONEncoder(config)
	} else {
		encoder = zapcore.NewConsoleEncoder(config)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return nil
}

// Get returns the global logger instance.
func Get() *zap.Logger {
	if globalLogger == nil {
		// Fallback to a no-op logger if not initialized
		return zap.NewNop()
	}
	return globalLogger
}

// WithContext returns a logger with correlation ID from context.
func WithContext(ctx context.Context) *zap.Logger {
	logger := Get()
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		logger = logger.With(zap.String("correlation_id", correlationID))
	}
	return logger
}

// WithField returns a logger with an additional field.
func WithField(key string, value interface{}) *zap.Logger {
	return Get().With(zap.Any(key, value))
}

// WithFields returns a logger with additional fields.
func WithFields(fields map[string]interface{}) *zap.Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return Get().With(zapFields...)
}

// SetCorrelationID adds a correlation ID to the context.
func SetCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from context.
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// GenerateCorrelationID generates a new correlation ID.
func GenerateCorrelationID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of specified length.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Sync flushes any buffered log entries.
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// Info logs an info level message.
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Error logs an error level message.
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Debug logs a debug level message.
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Warn logs a warning level message.
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Fatal logs a fatal level message and exits.
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}
