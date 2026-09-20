package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/shared/pkg/logger"
	"zuri/shared/pkg/redis"
)

// AIDedupMiddleware returns middleware that deduplicates AI requests.
func AIDedupMiddleware(redisClient *redis.Client, pathPrefixes []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method != http.MethodPost && method != http.MethodPut {
				return next(c)
			}

			path := c.Request().URL.Path
			if !isAIEndpoint(path, pathPrefixes) {
				return next(c)
			}

			hash, err := computeRequestHash(c)
			if err != nil {
				logger.Error("failed to compute request hash for dedup", zap.Error(err))
				return next(c)
			}

			key := "ai_dedup:" + hash
			ctx := c.Request().Context()

			cached, err := redisClient.Get(ctx, key)
			if err != nil {
				logger.Error("ai dedup redis get failed", zap.Error(err))
				return next(c)
			}

			if cached == "processing" {
				c.Response().Header().Set("X-AI-Dedup", "processing")
				return echo.NewHTTPError(http.StatusTooManyRequests, "request is already being processed")
			}

			if cached != "" {
				var resp cachedResponse
				if err := json.Unmarshal([]byte(cached), &resp); err == nil {
					if resp.ContentType != "" {
						c.Response().Header().Set("Content-Type", resp.ContentType)
					}
					c.Response().Header().Set("X-AI-Dedup", "hit")
					c.Response().WriteHeader(resp.Status)
					_, writeErr := c.Response().Write(resp.Body)
					return writeErr
				}
				logger.Error("failed to unmarshal cached ai dedup response", zap.Error(err))
			}

			// Mark as processing
			if err := redisClient.Set(ctx, key, "processing", 5*time.Minute); err != nil {
				logger.Error("failed to set ai dedup processing marker", zap.Error(err))
			}

			// Capture response
			recorder := &bodyCaptureWriter{
				ResponseWriter: c.Response().Writer,
				body:           &bytes.Buffer{},
			}
			c.Response().Writer = recorder

			handlerErr := next(c)

			status := c.Response().Status
			if status == 0 {
				status = http.StatusOK
			}

			if handlerErr == nil && status >= 200 && status < 300 {
				resp := cachedResponse{
					Status:      status,
					ContentType: c.Response().Header().Get("Content-Type"),
					Body:        recorder.body.Bytes(),
				}
				respJSON, marshalErr := json.Marshal(resp)
				if marshalErr == nil {
					if err := redisClient.Set(ctx, key, string(respJSON), 24*time.Hour); err != nil {
						logger.Error("failed to cache ai dedup response", zap.Error(err))
					}
				}
			} else {
				// Remove processing marker on error or non-2xx to allow retry
				_ = redisClient.Delete(ctx, key)
			}

			return handlerErr
		}
	}
}

type cachedResponse struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type,omitempty"`
	Body        []byte `json:"body"`
}

type bodyCaptureWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func computeRequestHash(c echo.Context) (string, error) {
	contentType := c.Request().Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		return computeMultipartHash(c)
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return "", err
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(body))

	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func computeMultipartHash(c echo.Context) (string, error) {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return "", err
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(body))

	_, params, err := mime.ParseMediaType(c.Request().Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}

	boundary := params["boundary"]
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	h := sha256.New()

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		h.Write([]byte(part.FormName()))
		h.Write([]byte{0})

		if filename := part.FileName(); filename != "" {
			h.Write([]byte(filename))
			h.Write([]byte{0})
		}

		if _, err := io.Copy(h, part); err != nil {
			part.Close()
			return "", err
		}
		part.Close()
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
