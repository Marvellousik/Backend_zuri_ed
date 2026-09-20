package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	
	"zuri/shared/pkg/logger"
)

// ReverseProxy handles proxying requests to upstream services.
type ReverseProxy struct {
	client            *http.Client
	circuitBreakers   map[string]*CircuitBreaker
	defaultCBThreshold int
	defaultCBTimeout   time.Duration
	internalAPIKey    string
}

// NewReverseProxy creates a new reverse proxy.
func NewReverseProxy(cbThreshold int, cbTimeout time.Duration, internalAPIKey string) *ReverseProxy {
	return &ReverseProxy{
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		circuitBreakers:    make(map[string]*CircuitBreaker),
		defaultCBThreshold: cbThreshold,
		defaultCBTimeout:   cbTimeout,
		internalAPIKey:     internalAPIKey,
	}
}

// ProxyRequest proxies a request to the target service.
func (p *ReverseProxy) ProxyRequest(c echo.Context, targetURL string, serviceName string, injectUserID bool) error {
	ctx := c.Request().Context()
	
	// Get or create circuit breaker for this service
	cb := p.getCircuitBreaker(serviceName)
	
	// Execute with circuit breaker protection
	err := cb.Execute(func() error {
		return p.doProxyRequest(ctx, c, targetURL, injectUserID)
	})
	
	if err == ErrCircuitOpen {
		logger.Warn("circuit breaker open, rejecting request",
			zap.String("service", serviceName),
		)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "service temporarily unavailable")
	}
	
	return err
}

// doProxyRequest performs the actual proxying.
func (p *ReverseProxy) doProxyRequest(ctx context.Context, c echo.Context, targetURL string, injectUserID bool) error {
	// Parse target URL
	target, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}
	
	// Create new request - must clear RequestURI as it's not allowed in client requests
	req := c.Request().Clone(ctx)
	req.RequestURI = "" // Clear RequestURI - only valid for incoming server requests
	
	// Update URL to point to target
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	
	// Clean up path to prevent double slashes
	targetPath := target.Path
	if targetPath == "/" {
		targetPath = ""
	}
	req.URL.Path = targetPath + req.URL.Path
	req.Host = target.Host
	
	// Remove hop-by-hop headers but PRESERVE Content-Type for multipart/form-data
	contentType := req.Header.Get("Content-Type")
	removeHopByHopHeaders(req.Header)
	
	// Restore Content-Type if it was multipart/form-data (needed for file uploads)
	if strings.Contains(contentType, "multipart/form-data") {
		req.Header.Set("Content-Type", contentType)
	}
	
	// Inject X-User-ID and X-User-Role headers if authenticated
	if injectUserID {
		if userID := c.Get("user_id"); userID != nil {
			if uidStr, ok := userID.(string); ok && uidStr != "" {
				req.Header.Set("X-User-ID", uidStr)
			}
		}
		if role := c.Get("role"); role != nil {
			if roleStr, ok := role.(string); ok && roleStr != "" {
				req.Header.Set("X-User-Role", roleStr)
			}
		}
		if tokenID := c.Get("token_id"); tokenID != nil {
			if tidStr, ok := tokenID.(string); ok && tidStr != "" {
				req.Header.Set("X-Token-ID", tidStr)
			}
		}
		if instID := c.Get("institution_id"); instID != nil {
			if instIDStr, ok := instID.(string); ok && instIDStr != "" {
				req.Header.Set("X-Institution-ID", instIDStr)
			}
		}
	}
	
	// Forward correlation ID
	if correlationID := req.Header.Get("X-Correlation-ID"); correlationID != "" {
		req.Header.Set("X-Correlation-ID", correlationID)
	}
	
	// Inject internal API key so upstream services can validate the request came from the gateway
	req.Header.Set("X-Internal-Key", p.internalAPIKey)
	
	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Copy headers from response
	for key, values := range resp.Header {
		for _, value := range values {
			c.Response().Header().Add(key, value)
		}
	}
	
	// Set status code
	c.Response().WriteHeader(resp.StatusCode)
	
	// Copy body
	_, err = io.Copy(c.Response().Writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}
	
	return nil
}

// ProxyWebSocket proxies a WebSocket connection to the target service.
func (p *ReverseProxy) ProxyWebSocket(c echo.Context, targetURL string, injectUserID bool) error {
	correlationID := c.Request().Header.Get("X-Correlation-ID")

	logger.Info("[WS] ProxyWebSocket called",
		zap.String("target_url", targetURL),
		zap.String("client_remote_addr", c.Request().RemoteAddr),
		zap.String("request_path", c.Request().URL.Path),
		zap.String("raw_query", c.Request().URL.RawQuery),
		zap.String("correlation_id", correlationID),
	)

	// Parse target URL and switch to ws:// or wss://
	target, err := url.Parse(targetURL)
	if err != nil {
		logger.Error("[WS] failed to parse target URL",
			zap.String("target_url", targetURL),
			zap.Error(err),
		)
		return fmt.Errorf("invalid target URL: %w", err)
	}
	switch target.Scheme {
	case "http":
		target.Scheme = "ws"
	case "https":
		target.Scheme = "wss"
	}

	logger.Info("[WS] resolved upstream target",
		zap.String("upstream_ws_url", target.String()),
		zap.String("correlation_id", correlationID),
	)

	// Build dialer with same timeout as HTTP client
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	// Prepare headers to forward
	headers := http.Header{}
	for key, values := range c.Request().Header {
		// Skip connection-specific and WebSocket handshake headers to avoid duplicate header errors in dialer
		canonicalKey := http.CanonicalHeaderKey(key)
		if canonicalKey == "Upgrade" ||
			canonicalKey == "Connection" ||
			canonicalKey == "Sec-Websocket-Key" ||
			canonicalKey == "Sec-Websocket-Version" ||
			canonicalKey == "Sec-Websocket-Extensions" {
			logger.Debug("[WS] skipping hop-by-hop header",
				zap.String("header", canonicalKey),
				zap.String("correlation_id", correlationID),
			)
			continue
		}
		for _, value := range values {
			headers.Add(key, value)
		}
	}

	// Inject X-User-ID and X-User-Role headers if authenticated
	if injectUserID {
		if userID := c.Get("user_id"); userID != nil {
			headers.Set("X-User-ID", userID.(string))
			logger.Debug("[WS] injected X-User-ID",
				zap.String("user_id", userID.(string)),
				zap.String("correlation_id", correlationID),
			)
		} else {
			logger.Warn("[WS] injectUserID=true but user_id not found in context",
				zap.String("correlation_id", correlationID),
			)
		}
		if role := c.Get("role"); role != nil {
			if roleStr, ok := role.(string); ok && roleStr != "" {
				headers.Set("X-User-Role", roleStr)
				logger.Debug("[WS] injected X-User-Role",
					zap.String("role", roleStr),
					zap.String("correlation_id", correlationID),
				)
			}
		}
		if tokenID := c.Get("token_id"); tokenID != nil {
			if tidStr, ok := tokenID.(string); ok && tidStr != "" {
				headers.Set("X-Token-ID", tidStr)
			}
		}
		if instID := c.Get("institution_id"); instID != nil {
			if instIDStr, ok := instID.(string); ok && instIDStr != "" {
				headers.Set("X-Institution-ID", instIDStr)
			}
		}
	}

	// Forward correlation ID
	if correlationID != "" {
		headers.Set("X-Correlation-ID", correlationID)
	}

	// Inject internal API key
	headers.Set("X-Internal-Key", p.internalAPIKey)

	// Copy query parameters
	if c.Request().URL.RawQuery != "" {
		target.RawQuery = c.Request().URL.RawQuery
	}

	// Log the final headers being sent upstream (redact sensitive values)
	logger.Debug("[WS] forwarding headers to upstream",
		zap.Strings("header_keys", func() []string {
			keys := make([]string, 0, len(headers))
			for k := range headers {
				keys = append(keys, k)
			}
			return keys
		}()),
		zap.String("correlation_id", correlationID),
	)

	// Dial upstream
	logger.Info("[WS] dialing upstream",
		zap.String("upstream_ws_url", target.String()),
		zap.String("correlation_id", correlationID),
	)
	upstreamConn, dialResp, err := dialer.Dial(target.String(), headers)
	if err != nil {
		if dialResp != nil {
			body, _ := io.ReadAll(dialResp.Body)
			dialResp.Body.Close()
			logger.Error("[WS] upstream dial failed with HTTP response",
				zap.String("upstream_ws_url", target.String()),
				zap.Int("status_code", dialResp.StatusCode),
				zap.String("response_body", string(body)),
				zap.String("correlation_id", correlationID),
				zap.Error(err),
			)
			return echo.NewHTTPError(dialResp.StatusCode, fmt.Sprintf("upstream WebSocket connection failed: %s", string(body)))
		}
		logger.Error("[WS] upstream dial failed with no HTTP response (network/timeout)",
			zap.String("upstream_ws_url", target.String()),
			zap.String("correlation_id", correlationID),
			zap.Error(err),
		)
		return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("upstream WebSocket connection failed: %v", err))
	}
	defer upstreamConn.Close()

	logger.Info("[WS] upstream dial succeeded",
		zap.String("upstream_ws_url", target.String()),
		zap.String("correlation_id", correlationID),
	)

	// Upgrade client connection
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	logger.Info("[WS] upgrading client connection",
		zap.String("client_remote_addr", c.Request().RemoteAddr),
		zap.String("correlation_id", correlationID),
	)
	clientConn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		logger.Error("[WS] failed to upgrade client connection",
			zap.String("client_remote_addr", c.Request().RemoteAddr),
			zap.String("correlation_id", correlationID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to upgrade client connection: %w", err)
	}
	defer clientConn.Close()

	logger.Info("[WS] tunnel established — starting bidirectional copy",
		zap.String("client_remote_addr", c.Request().RemoteAddr),
		zap.String("upstream", target.String()),
		zap.String("correlation_id", correlationID),
	)

	// Copy messages bidirectionally
	errChan := make(chan error, 2)

	go func() {
		for {
			msgType, msg, err := upstreamConn.ReadMessage()
			if err != nil {
				logger.Warn("[WS] upstream→client copy stopped",
					zap.String("correlation_id", correlationID),
					zap.Error(err),
				)
				errChan <- err
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				logger.Warn("[WS] upstream→client write failed",
					zap.String("correlation_id", correlationID),
					zap.Error(err),
				)
				errChan <- err
				return
			}
		}
	}()

	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				logger.Warn("[WS] client→upstream copy stopped",
					zap.String("correlation_id", correlationID),
					zap.Error(err),
				)
				errChan <- err
				return
			}
			if err := upstreamConn.WriteMessage(msgType, msg); err != nil {
				logger.Warn("[WS] client→upstream write failed",
					zap.String("correlation_id", correlationID),
					zap.Error(err),
				)
				errChan <- err
				return
			}
		}
	}()

	// Block until one side closes
	closeErr := <-errChan
	logger.Info("[WS] tunnel closed",
		zap.String("client_remote_addr", c.Request().RemoteAddr),
		zap.String("upstream", target.String()),
		zap.String("correlation_id", correlationID),
		zap.NamedError("close_reason", closeErr),
	)
	return nil
}

// getCircuitBreaker gets or creates a circuit breaker for a service.
func (p *ReverseProxy) getCircuitBreaker(serviceName string) *CircuitBreaker {
	if cb, exists := p.circuitBreakers[serviceName]; exists {
		return cb
	}
	
	cb := NewCircuitBreaker(serviceName, p.defaultCBThreshold, p.defaultCBTimeout)
	p.circuitBreakers[serviceName] = cb
	return cb
}

// removeHopByHopHeaders removes headers that should not be forwarded.
func removeHopByHopHeaders(header http.Header) {
	hopByHop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	
	for _, h := range hopByHop {
		header.Del(h)
	}
}

// HealthCheck checks if a service is healthy.
func (p *ReverseProxy) HealthCheck(serviceURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL+"/health", nil)
	if err != nil {
		return err
	}
	
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}
