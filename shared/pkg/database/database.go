// Package database provides PostgreSQL connection pooling, retry logic, and migration runner.
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration.
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultConfig returns a default database configuration.
func DefaultConfig(dsn string) *Config {
	return &Config{
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
		LogLevel:        logger.Warn,
	}
}

// DB wraps gorm.DB with additional functionality.
type DB struct {
	*gorm.DB
}

// New creates a new database connection with retry logic.
func New(cfg *Config) (*DB, error) {
	var db *gorm.DB
	var err error

	// Retry logic for initial connection
	maxRetries := 5
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
			Logger: logger.Default.LogMode(cfg.LogLevel),
		})
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return &DB{db}, nil
}

// Transaction executes a function within a database transaction.
func (db *DB) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return db.DB.WithContext(ctx).Transaction(fn)
}

// Ping checks if the database connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the database connection.
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// RunMigrations runs database migrations using golang-migrate.
func (db *DB) RunMigrations(migrationsPath string) error {
	// This is a placeholder for migration logic
	// In production, you would use golang-migrate or similar
	// For now, we'll rely on GORM's AutoMigrate for development
	return nil
}

// HealthCheck performs a health check on the database.
func (db *DB) HealthCheck(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// WithUserFilter returns a scoped DB that filters by user_id.
func (db *DB) WithUserFilter(userID string) *gorm.DB {
	return db.DB.Where("user_id = ?", userID)
}

// SoftDeleteScope returns a GORM scope that excludes soft-deleted records.
func SoftDeleteScope(db *gorm.DB) *gorm.DB {
	return db.Where("deleted_at IS NULL")
}

// WithPagination returns a scoped DB with pagination applied.
func WithPagination(page, pageSize int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

// Retry executes a database operation with retry logic.
func Retry(ctx context.Context, maxRetries int, fn func() error) error {
	var err error
	delay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
}
