// Package clients provides HTTP clients for Python microservices.
package clients

import (
	"time"
)

// ChatRequest represents a chat request to the AI Orchestrator
type ChatRequest struct {
	Query           string   `json:"query"`
	UserID          string   `json:"user_id"`
	ContextChunks   []string `json:"context_chunks,omitempty"`
	MaterialID      *string  `json:"material_id,omitempty"`
	ConversationID  *string  `json:"conversation_id,omitempty"`
}

// ChatResponse represents a chat response from the AI Orchestrator
type ChatResponse struct {
	Response       string   `json:"response"`
	ConversationID string   `json:"conversation_id"`
	TokensUsed     int      `json:"tokens_used"`
	Model          string   `json:"model"`
	Sources        []string `json:"sources"`
}

// RetrieveRequest represents a retrieval request
type RetrieveRequest struct {
	Query      string `json:"query"`
	UserID     string `json:"user_id"`
	MaterialID string `json:"material_id,omitempty"`
	TopK       int    `json:"top_k"`
}

// ChunkResult represents a retrieved chunk
type ChunkResult struct {
	ChunkID         string  `json:"chunk_id"`
	MaterialID      string  `json:"material_id"`
	ChunkText       string  `json:"chunk_text"`
	SimilarityScore float64 `json:"similarity_score"`
	ChunkIndex      int     `json:"chunk_index"`
}

// RetrieveResponse represents a retrieval response
type RetrieveResponse struct {
	Query                 string        `json:"query"`
	QueryEmbeddingPreview []float64     `json:"query_embedding_preview"`
	Results               []ChunkResult `json:"results"`
	Cached                bool          `json:"cached"`
	Note                  string        `json:"note"`
}

// ProcessDocumentRequest represents a document processing request
type ProcessDocumentRequest struct {
	MaterialID string `json:"material_id"`
	UserID     string `json:"user_id"`
	FileURL    string `json:"file_url"`
}

// ProcessDocumentResponse represents a document processing response
type ProcessDocumentResponse struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	ChunksCreated   int    `json:"chunks_created"`
	StorageMethod   string `json:"storage_method"`
}

// TranscriptionRequest represents a speech-to-text request (multipart form data)
type TranscriptionRequest struct {
	AudioData   []byte
	ContentType string
	Language    string
}

// TranscriptionResponse represents a speech-to-text response
type TranscriptionResponse struct {
	Text            string  `json:"text"`
	Confidence      float64 `json:"confidence"`
	Language        string  `json:"language"`
	OriginalFormat  string  `json:"original_format"`
}

// AIUsageLog represents AI usage data for analytics
type AIUsageLog struct {
	UserID        string    `json:"user_id"`
	InteractionType string  `json:"interaction_type"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	LatencyMs     int64     `json:"latency_ms"`
	Model         string    `json:"model"`
	Timestamp     time.Time `json:"timestamp"`
}

// ProcessingStatusUpdate represents a material processing status update
type ProcessingStatusUpdate struct {
	MaterialID    string `json:"material_id"`
	Status        string `json:"status"` // "completed", "failed", "processing"
	ChunksCreated int    `json:"chunks_created,omitempty"`
	Error         string `json:"error,omitempty"`
}
