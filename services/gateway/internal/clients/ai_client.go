// Package clients provides HTTP clients for external services.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
	"zuri/services/gateway/internal/config"
	"zuri/shared/pkg/logger"
)

// Log is the package-level logger
var Log = logger.Get()

// AIClient provides methods to interact with the FastAPI AI service.
// It handles multipart uploads, SSE streaming, and authentication injection.
type AIClient struct {
	baseURL     string
	internalKey string
	httpClient  *http.Client
	logger      *zap.Logger
}

// NewAIClient creates a new AI service client with configured timeouts and connection pooling.
func NewAIClient(cfg *config.Config) *AIClient {
	return &AIClient{
		baseURL:     cfg.AIServiceURL,
		internalKey: cfg.InternalAPIKey,
		httpClient: &http.Client{
			Timeout: cfg.AIServiceTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: Log,
	}
}

// VocabTerm represents a vocabulary term in reading analysis.
type VocabTerm struct {
	Term           string `json:"term"`
	Definition     string `json:"definition"`
	ContextSnippet string `json:"context_snippet"`
}

// ReadingResponse represents the response from document analysis.
type ReadingResponse struct {
	SessionID     string      `json:"session_id"`
	UserID        string      `json:"user_id"`
	SummaryType   string      `json:"summary_type"`
	Summary       string      `json:"summary"`
	VocabTerms    []VocabTerm `json:"vocab_terms"`
	TTSAudioB64   string      `json:"tts_audio_b64"`
	AudioMimeType string      `json:"audio_mime_type"`
	Voice         string      `json:"voice"`
}

// Flashcard represents a single flashcard.
type Flashcard struct {
	Front string `json:"front"`
	Back  string `json:"back"`
	Topic string `json:"topic"`
}

// FlashcardResponse represents the response from flashcard generation.
type FlashcardResponse struct {
	SessionID     string      `json:"session_id"`
	UserID        string      `json:"user_id"`
	Filename      string      `json:"filename"`
	NumRequested  int         `json:"num_requested"`
	NumGenerated  int         `json:"num_generated"`
	Flashcards    []Flashcard `json:"flashcards"`
}

// MCQOptions represents options for multiple choice questions.
type MCQOptions struct {
	A string `json:"A"`
	B string `json:"B"`
	C string `json:"C"`
	D string `json:"D"`
}

// MultipleChoiceQuestion represents an MCQ.
type MultipleChoiceQuestion struct {
	Question      string   `json:"question"`
	Options       MCQOptions `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
	Topic         string   `json:"topic"`
}

// TheoryQuestion represents a theory quiz question.
type TheoryQuestion struct {
	Question     string   `json:"question"`
	ModelAnswer  string   `json:"model_answer"`
	MarkingGuide []string `json:"marking_guide"`
	Marks        int      `json:"marks"`
	Topic        string   `json:"topic"`
}

// QuizResponse represents the response from quiz generation.
type QuizResponse struct {
	SessionID     string                   `json:"session_id"`
	UserID        string                   `json:"user_id"`
	Filename      string                   `json:"filename"`
	QuizType      string                   `json:"quiz_type"`
	NumRequested  int                      `json:"num_requested"`
	NumGenerated  int                      `json:"num_generated"`
	Questions     []MultipleChoiceQuestion `json:"questions"`
	// Note: For theory quizzes, Questions would be []TheoryQuestion
	// The caller should handle type assertion if needed
}

// NotesResponse represents the response from note generation.
type NotesResponse struct {
	SessionID       string `json:"session_id"`
	UserID          string `json:"user_id"`
	StructuredNotes string `json:"structured_notes"`
}

// SessionSummary represents a session metadata entry.
type SessionSummary struct {
	SessionID    string `json:"session_id"`
	SessionType  string `json:"session_type"`
	Filename     string `json:"filename"`
	CreatedAt    string `json:"created_at"`
	QuizType     string `json:"quiz_type,omitempty"`
	NumCards     int    `json:"num_cards,omitempty"`
	NumQuestions int    `json:"num_questions,omitempty"`
}

// doRequest executes an HTTP request with retry logic and error mapping.
func (c *AIClient) doRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	backoffs := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	for attempt := 0; attempt <= len(backoffs); attempt++ {
		req.Header.Set("X-Internal-Key", c.internalKey)
		start := time.Now()
		resp, err = c.httpClient.Do(req)
		duration := time.Since(start)

		if err == nil && resp.StatusCode < 500 {
			// Success or client error (don't retry)
			c.logger.Info("AI request completed",
				zap.String("method", req.Method),
				zap.String("url", req.URL.Path),
				zap.Int("status", resp.StatusCode),
				zap.Duration("duration", duration),
			)
			return resp, nil
		}

		if err != nil {
			c.logger.Warn("AI request failed",
				zap.String("method", req.Method),
				zap.String("url", req.URL.Path),
				zap.Error(err),
				zap.Int("attempt", attempt+1),
			)
		} else {
			c.logger.Warn("AI request returned server error",
				zap.String("method", req.Method),
				zap.String("url", req.URL.Path),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt+1),
			)
		}

		// Don't retry after last attempt
		if attempt < len(backoffs) {
			time.Sleep(backoffs[attempt])
			// Recreate request body for retry
			if req.Body != nil && req.GetBody != nil {
				req.Body, _ = req.GetBody()
			}
		}
	}

	return resp, err
}

// mapErrorStatus maps AI service status codes to appropriate HTTP status codes.
func (c *AIClient) mapErrorStatus(aiStatusCode int) int {
	switch aiStatusCode {
	case http.StatusUnsupportedMediaType: // 415
		return http.StatusUnsupportedMediaType
	case http.StatusUnprocessableEntity: // 422
		return http.StatusBadRequest
	case http.StatusNotFound: // 404
		return http.StatusNotFound
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return http.StatusBadGateway
	default:
		return aiStatusCode
	}
}

// Transcribe sends an audio file to the AI service for transcription.
// Returns an io.ReadCloser for SSE streaming - caller must close it.
func (c *AIClient) Transcribe(ctx context.Context, audioFile io.Reader, filename, userID string, sessionID *string) (io.ReadCloser, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add user_id field (REQUIRED for AI ownership)
	if err := writer.WriteField("user_id", userID); err != nil {
		return nil, fmt.Errorf("failed to write user_id field: %w", err)
	}

	// Add session_id if provided
	if sessionID != nil && *sessionID != "" {
		if err := writer.WriteField("session_id", *sessionID); err != nil {
			return nil, fmt.Errorf("failed to write session_id field: %w", err)
		}
	}

	// Add file
	part, err := writer.CreateFormFile("audio", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("failed to copy audio file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/writing/transcribe", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "text/event-stream")

	// Add idempotency key
	idempotencyKey := userID
	if sessionID != nil {
		idempotencyKey = *sessionID
	}
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("transcription failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp.Body, nil
}

// GenerateNotes sends raw transcript to AI for note generation.
func (c *AIClient) GenerateNotes(ctx context.Context, sessionID, rawText, subject, userID string, save bool) (*NotesResponse, error) {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"raw_text":   rawText,
		"subject":    subject,
		"user_id":    userID,
		"save":       save,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/writing/notes", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", sessionID)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("notes generation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notes generation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var notesResp NotesResponse
	if err := json.NewDecoder(resp.Body).Decode(&notesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &notesResp, nil
}

// AnalyzeDocument uploads a document for analysis (summary + TTS).
func (c *AIClient) AnalyzeDocument(ctx context.Context, file io.Reader, filename, userID, summaryType, voice string) (*ReadingResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add user_id field (REQUIRED)
	if err := writer.WriteField("user_id", userID); err != nil {
		return nil, fmt.Errorf("failed to write user_id field: %w", err)
	}

	// Add summary_type
	if summaryType == "" {
		summaryType = "concise"
	}
	if err := writer.WriteField("summary_type", summaryType); err != nil {
		return nil, fmt.Errorf("failed to write summary_type field: %w", err)
	}

	// Add voice
	if voice == "" {
		voice = "Zephyr"
	}
	if err := writer.WriteField("voice", voice); err != nil {
		return nil, fmt.Errorf("failed to write voice field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/reading/analyse", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Generate idempotency key from filename + userID
	idempotencyKey := fmt.Sprintf("%s-%s-%d", userID, filename, time.Now().Unix())
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("document analysis request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("document analysis failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var readingResp ReadingResponse
	if err := json.NewDecoder(resp.Body).Decode(&readingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &readingResp, nil
}

// GenerateFlashcards generates flashcards from uploaded notes.
func (c *AIClient) GenerateFlashcards(ctx context.Context, file io.Reader, filename, userID string, numCards int) (*FlashcardResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add user_id field (REQUIRED)
	if err := writer.WriteField("user_id", userID); err != nil {
		return nil, fmt.Errorf("failed to write user_id field: %w", err)
	}

	// Add num_cards
	if numCards <= 0 {
		numCards = 10
	}
	if err := writer.WriteField("num_cards", strconv.Itoa(numCards)); err != nil {
		return nil, fmt.Errorf("failed to write num_cards field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/study/flashcards", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	idempotencyKey := fmt.Sprintf("%s-%s-%d", userID, filename, time.Now().Unix())
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("flashcard generation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("flashcard generation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var flashcardResp FlashcardResponse
	if err := json.NewDecoder(resp.Body).Decode(&flashcardResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &flashcardResp, nil
}

// GenerateQuiz generates a quiz from uploaded notes.
func (c *AIClient) GenerateQuiz(ctx context.Context, file io.Reader, filename, userID, quizType string, numQuestions int) (*QuizResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add user_id field (REQUIRED)
	if err := writer.WriteField("user_id", userID); err != nil {
		return nil, fmt.Errorf("failed to write user_id field: %w", err)
	}

	// Add quiz_type
	if quizType == "" {
		quizType = "multiple_choice"
	}
	if err := writer.WriteField("quiz_type", quizType); err != nil {
		return nil, fmt.Errorf("failed to write quiz_type field: %w", err)
	}

	// Add num_questions
	if numQuestions <= 0 {
		numQuestions = 5
	}
	if err := writer.WriteField("num_questions", strconv.Itoa(numQuestions)); err != nil {
		return nil, fmt.Errorf("failed to write num_questions field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/study/quiz", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	idempotencyKey := fmt.Sprintf("%s-%s-%s-%d", userID, filename, quizType, time.Now().Unix())
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("quiz generation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("quiz generation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var quizResp QuizResponse
	if err := json.NewDecoder(resp.Body).Decode(&quizResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &quizResp, nil
}

// GetSession retrieves a session by ID and type. Returns generic interface{} - caller should type assert.
func (c *AIClient) GetSession(ctx context.Context, sessionID, userID, sessionType string) (interface{}, error) {
	var endpoint string
	switch sessionType {
	case "notes":
		endpoint = fmt.Sprintf("/writing/notes/session/%s?user_id=%s", sessionID, url.QueryEscape(userID))
	case "reading":
		endpoint = fmt.Sprintf("/reading/session/%s?user_id=%s", sessionID, url.QueryEscape(userID))
	case "flashcard":
		endpoint = fmt.Sprintf("/study/flashcards/session/%s?user_id=%s", sessionID, url.QueryEscape(userID))
	case "quiz":
		endpoint = fmt.Sprintf("/study/quiz/session/%s?user_id=%s", sessionID, url.QueryEscape(userID))
	default:
		return nil, fmt.Errorf("unknown session type: %s", sessionType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("get session request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("session not found")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get session failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode based on session type
	var result interface{}
	switch sessionType {
	case "notes":
		var notes struct {
			SessionID       string `json:"session_id"`
			UserID          string `json:"user_id"`
			Subject         string `json:"subject"`
			CreatedAt       string `json:"created_at"`
			StructuredNotes string `json:"structured_notes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
			return nil, fmt.Errorf("failed to decode notes response: %w", err)
		}
		result = notes
	case "reading":
		var reading struct {
			SessionID    string      `json:"session_id"`
			UserID       string      `json:"user_id"`
			Filename     string      `json:"filename"`
			CreatedAt    string      `json:"created_at"`
			SummaryType  string      `json:"summary_type"`
			Summary      string      `json:"summary"`
			VocabTerms   []VocabTerm `json:"vocab_terms"`
			TTSAudioB64  string      `json:"tts_audio_b64"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&reading); err != nil {
			return nil, fmt.Errorf("failed to decode reading response: %w", err)
		}
		result = reading
	case "flashcard":
		var flashcards struct {
			SessionID  string      `json:"session_id"`
			UserID     string      `json:"user_id"`
			Filename   string      `json:"filename"`
			CreatedAt  string      `json:"created_at"`
			NumCards   int         `json:"num_cards"`
			Flashcards []Flashcard `json:"flashcards"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&flashcards); err != nil {
			return nil, fmt.Errorf("failed to decode flashcards response: %w", err)
		}
		result = flashcards
	case "quiz":
		var quiz struct {
			SessionID    string                   `json:"session_id"`
			UserID       string                   `json:"user_id"`
			Filename     string                   `json:"filename"`
			CreatedAt    string                   `json:"created_at"`
			QuizType     string                   `json:"quiz_type"`
			NumQuestions int                      `json:"num_questions"`
			Questions    []MultipleChoiceQuestion `json:"questions"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&quiz); err != nil {
			return nil, fmt.Errorf("failed to decode quiz response: %w", err)
		}
		result = quiz
	}

	return result, nil
}

// GetStudyHistory retrieves all flashcard and quiz sessions for a user.
func (c *AIClient) GetStudyHistory(ctx context.Context, userID string) ([]SessionSummary, error) {
	endpoint := fmt.Sprintf("/study/history?user_id=%s", url.QueryEscape(userID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("get study history request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get study history failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var history []SessionSummary
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return history, nil
}

// GetNotesHistory retrieves all notes sessions for a user.
func (c *AIClient) GetNotesHistory(ctx context.Context, userID string) ([]SessionSummary, error) {
	endpoint := fmt.Sprintf("/writing/notes/history?user_id=%s", url.QueryEscape(userID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("get notes history request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get notes history failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var history []SessionSummary
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return history, nil
}
