// Package handlers provides HTTP handlers for AI service proxy endpoints.
package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/services/gateway/internal/clients"
	"zuri/shared/pkg/logger"
)

// AIHandler handles AI service proxy endpoints.
type AIHandler struct {
	aiClient *clients.AIClient
	logger   *zap.Logger
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(aiClient *clients.AIClient) *AIHandler {
	return &AIHandler{
		aiClient: aiClient,
		logger:   logger.Get(),
	}
}

// getUserID extracts the user ID from the Echo context (set by JWT middleware).
func (h *AIHandler) getUserID(c echo.Context) string {
	userID := c.Get("user_id")
	if userID == nil {
		return ""
	}
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}

// ==================== WRITING ASSISTANT ====================

// Transcribe handles live audio transcription via SSE streaming.
// POST /api/writing/transcribe -> AI POST /writing/transcribe
func (h *AIHandler) Transcribe(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	// Parse multipart form (32MB max memory)
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		h.logger.Warn("failed to parse multipart form", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}

	// Get audio file
	file, header, err := c.Request().FormFile("audio")
	if err != nil {
		h.logger.Warn("missing audio file", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "missing audio file")
	}
	defer file.Close()

	// Get optional session_id
	var sessionID *string
	if sid := c.FormValue("session_id"); sid != "" {
		sessionID = &sid
	}

	// Call AI service - returns SSE stream
	stream, err := h.aiClient.Transcribe(c.Request().Context(), file, header.Filename, userID, sessionID)
	if err != nil {
		h.logger.Error("transcription failed", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "transcription service unavailable")
	}
	defer stream.Close()

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// Stream response to client
	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Warn("error reading from AI stream", zap.Error(err))
			break
		}
		if n > 0 {
			c.Response().Writer.Write(buf[:n])
			flusher.Flush()
		}
	}

	return nil
}

// GenerateNotesRequest represents the note generation request.
type GenerateNotesRequest struct {
	SessionID string `json:"session_id" validate:"required"`
	RawText   string `json:"raw_text" validate:"required"`
	Subject   string `json:"subject" default:"General"`
	Save      bool   `json:"save" default:"true"`
}

// GenerateNotes converts raw transcript into structured notes.
// POST /api/writing/notes -> AI POST /writing/notes
func (h *AIHandler) GenerateNotes(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	var req GenerateNotesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Subject == "" {
		req.Subject = "General"
	}

	resp, err := h.aiClient.GenerateNotes(c.Request().Context(), req.SessionID, req.RawText, req.Subject, userID, req.Save)
	if err != nil {
		h.logger.Error("notes generation failed", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "notes generation failed")
	}

	return c.JSON(http.StatusOK, resp)
}

// GetNotesSession retrieves a past notes session.
// GET /api/writing/notes/:id -> AI GET /writing/notes/session/:id?user_id=xxx
func (h *AIHandler) GetNotesSession(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing session ID")
	}

	session, err := h.aiClient.GetSession(c.Request().Context(), sessionID, userID, "notes")
	if err != nil {
		if err.Error() == "session not found" {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		h.logger.Error("failed to get notes session", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve session")
	}

	return c.JSON(http.StatusOK, session)
}

// ==================== READING ASSISTANT ====================

// AnalyzeDocument analyzes a document (summary + TTS).
// POST /api/reading/analyze -> AI POST /reading/analyse
func (h *AIHandler) AnalyzeDocument(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	// Parse multipart form (32MB max)
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		h.logger.Warn("failed to parse multipart form", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}

	// Get file
	file, header, err := c.Request().FormFile("file")
	if err != nil {
		h.logger.Warn("missing file", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "missing file")
	}
	defer file.Close()

	// Get options
	summaryType := c.FormValue("summary_type")
	if summaryType == "" {
		summaryType = "concise"
	}
	voice := c.FormValue("voice")
	if voice == "" {
		voice = "Zephyr"
	}

	resp, err := h.aiClient.AnalyzeDocument(c.Request().Context(), file, header.Filename, userID, summaryType, voice)
	if err != nil {
		h.logger.Error("document analysis failed", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "document analysis failed")
	}

	return c.JSON(http.StatusOK, resp)
}

// GetReadingSession retrieves a past reading session.
// GET /api/reading/:id -> AI GET /reading/session/:id?user_id=xxx
func (h *AIHandler) GetReadingSession(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing session ID")
	}

	session, err := h.aiClient.GetSession(c.Request().Context(), sessionID, userID, "reading")
	if err != nil {
		if err.Error() == "session not found" {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		h.logger.Error("failed to get reading session", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve session")
	}

	return c.JSON(http.StatusOK, session)
}

// ==================== STUDY BUDDY ====================

// GenerateFlashcards generates flashcards from uploaded notes.
// POST /api/study/flashcards -> AI POST /study/flashcards
func (h *AIHandler) GenerateFlashcards(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	// Parse multipart form
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		h.logger.Warn("failed to parse multipart form", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}

	// Get file
	file, header, err := c.Request().FormFile("file")
	if err != nil {
		h.logger.Warn("missing file", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "missing file")
	}
	defer file.Close()

	// Get num_cards
	numCardsStr := c.FormValue("num_cards")
	numCards := 10
	if numCardsStr != "" {
		if n, err := strconv.Atoi(numCardsStr); err == nil && n > 0 {
			numCards = n
		}
	}

	resp, err := h.aiClient.GenerateFlashcards(c.Request().Context(), file, header.Filename, userID, numCards)
	if err != nil {
		h.logger.Error("flashcard generation failed", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "flashcard generation failed")
	}

	return c.JSON(http.StatusOK, resp)
}

// GetFlashcardSession retrieves a past flashcard session.
// GET /api/study/flashcards/:id -> AI GET /study/flashcards/session/:id?user_id=xxx
func (h *AIHandler) GetFlashcardSession(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing session ID")
	}

	session, err := h.aiClient.GetSession(c.Request().Context(), sessionID, userID, "flashcard")
	if err != nil {
		if err.Error() == "session not found" {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		h.logger.Error("failed to get flashcard session", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve session")
	}

	return c.JSON(http.StatusOK, session)
}

// GenerateQuiz generates a quiz from uploaded notes.
// POST /api/study/quiz -> AI POST /study/quiz
func (h *AIHandler) GenerateQuiz(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	// Parse multipart form
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		h.logger.Warn("failed to parse multipart form", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}

	// Get file
	file, header, err := c.Request().FormFile("file")
	if err != nil {
		h.logger.Warn("missing file", zap.Error(err))
		return echo.NewHTTPError(http.StatusBadRequest, "missing file")
	}
	defer file.Close()

	// Get options
	quizType := c.FormValue("quiz_type")
	if quizType == "" {
		quizType = "multiple_choice"
	}

	numQuestionsStr := c.FormValue("num_questions")
	numQuestions := 5
	if numQuestionsStr != "" {
		if n, err := strconv.Atoi(numQuestionsStr); err == nil && n > 0 {
			numQuestions = n
		}
	}

	resp, err := h.aiClient.GenerateQuiz(c.Request().Context(), file, header.Filename, userID, quizType, numQuestions)
	if err != nil {
		h.logger.Error("quiz generation failed", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "quiz generation failed")
	}

	return c.JSON(http.StatusOK, resp)
}

// GetQuizSession retrieves a past quiz session.
// GET /api/study/quiz/:id -> AI GET /study/quiz/session/:id?user_id=xxx
func (h *AIHandler) GetQuizSession(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing session ID")
	}

	session, err := h.aiClient.GetSession(c.Request().Context(), sessionID, userID, "quiz")
	if err != nil {
		if err.Error() == "session not found" {
			return echo.NewHTTPError(http.StatusNotFound, "session not found")
		}
		h.logger.Error("failed to get quiz session", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve session")
	}

	return c.JSON(http.StatusOK, session)
}

// GetStudyHistory retrieves all study sessions (flashcards and quizzes).
// GET /api/study/history -> AI GET /study/history?user_id=xxx
func (h *AIHandler) GetStudyHistory(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	history, err := h.aiClient.GetStudyHistory(c.Request().Context(), userID)
	if err != nil {
		h.logger.Error("failed to get study history", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve history")
	}

	return c.JSON(http.StatusOK, history)
}

// GetNotesHistory retrieves all notes sessions.
// GET /api/writing/history -> AI GET /writing/notes/history?user_id=xxx
func (h *AIHandler) GetNotesHistory(c echo.Context) error {
	userID := h.getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user ID")
	}

	history, err := h.aiClient.GetNotesHistory(c.Request().Context(), userID)
	if err != nil {
		h.logger.Error("failed to get notes history", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve history")
	}

	return c.JSON(http.StatusOK, history)
}
