// Package clients provides HTTP clients for Python microservices with retry logic and circuit breaking.
package clients

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"zuri/services/gateway/internal/circuitbreaker"
)

// Config holds client configuration
type Config struct {
	BaseURL         string
	Timeout         time.Duration
	InternalAPIKey  string
	MaxRetries      int
	RetryBackoff    time.Duration
}

// DefaultConfigs for different services
func IngestionDefaultConfig() Config {
	return Config{
		Timeout:      10 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 2 * time.Second,
	}
}

func RetrievalDefaultConfig() Config {
	return Config{
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 2 * time.Second,
	}
}

func AudioDefaultConfig() Config {
	return Config{
		Timeout:      60 * time.Second,
		MaxRetries:   2,
		RetryBackoff: 5 * time.Second,
	}
}

func OrchestratorDefaultConfig() Config {
	return Config{
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 2 * time.Second,
	}
}

// Client is a generic HTTP client for Python services
type Client struct {
	config         Config
	httpClient     *http.Client
	circuitBreaker *circuitbreaker.CircuitBreaker
	serviceName    string
}

// NewClient creates a new Python service client
func NewClient(serviceName, baseURL, internalAPIKey string, config Config, useCircuitBreaker bool) *Client {
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	c := &Client{
		config: Config{
			BaseURL:        baseURL,
			Timeout:        config.Timeout,
			InternalAPIKey: internalAPIKey,
			MaxRetries:     config.MaxRetries,
			RetryBackoff:   config.RetryBackoff,
		},
		httpClient:  httpClient,
		serviceName: serviceName,
	}

	if useCircuitBreaker {
		cbConfig := circuitbreaker.DefaultConfig()
		cbConfig.OnStateChange = func(from, to circuitbreaker.State) {
			fmt.Printf("[CircuitBreaker] %s: %s -> %s\n", serviceName, from, to)
		}
		c.circuitBreaker = circuitbreaker.New(cbConfig)
	}

	return c
}

// doRequest executes an HTTP request with retries
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte, contentType string) (*http.Response, error) {
	url := c.config.BaseURL + path

	// Function to execute the request
	execute := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Internal-Key", c.config.InternalAPIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		return resp, nil
	}

	// Execute with circuit breaker if enabled
	if c.circuitBreaker != nil {
		var resp *http.Response
		err := c.circuitBreaker.Execute(ctx, func() error {
			var err error
			resp, err = execute()
			if err != nil {
				return err
			}
			// Check for HTTP errors that should count as failures
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}
			return nil
		})
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, err
		}
		return resp, err
	}

	// Execute without circuit breaker
	return execute()
}

// doWithRetry executes a request with exponential backoff retry logic
func (c *Client) doWithRetry(ctx context.Context, method, path string, body []byte, contentType string) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s...
			backoff := c.config.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err = c.doRequest(ctx, method, path, body, contentType)
		if err != nil {
			if err == circuitbreaker.ErrCircuitBreakerOpen {
				return nil, err
			}
			// Check if error is retryable
			if attempt < c.config.MaxRetries {
				continue
			}
			return nil, err
		}

		// Check HTTP status code
		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusAccepted:
			return resp, nil
		case http.StatusTooManyRequests:
			// Rate limited - retry
			resp.Body.Close()
			if attempt < c.config.MaxRetries {
				continue
			}
			return nil, fmt.Errorf("rate limited after %d retries", c.config.MaxRetries)
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound:
			// Don't retry client errors
			return resp, nil
		default:
			// Server error - retry
			if attempt < c.config.MaxRetries {
				resp.Body.Close()
				continue
			}
			return resp, nil
		}
	}

	return resp, err
}

// Health checks if the service is healthy
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.doWithRetry(ctx, http.MethodGet, "/health", nil, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetCircuitBreakerMetrics returns circuit breaker metrics (if enabled)
func (c *Client) GetCircuitBreakerMetrics() circuitbreaker.Metrics {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.Metrics()
	}
	return circuitbreaker.Metrics{State: "DISABLED"}
}
