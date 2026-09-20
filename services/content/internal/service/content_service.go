// Package service provides business logic for content management.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"zuri/services/content/internal/model"
	"zuri/services/content/internal/repository"
)

var (
	ErrCourseNotFound    = errors.New("course not found")
	ErrMaterialNotFound  = errors.New("material not found")
	ErrQuizNotFound      = errors.New("quiz not found")
	ErrQuestionNotFound  = errors.New("question not found")
	ErrFlashcardNotFound = errors.New("flashcard not found")
	ErrUnauthorized      = errors.New("unauthorized access")
	ErrInvalidInput      = errors.New("invalid input")
)

// ContentService defines the content management interface.
type ContentService interface {
	// Course operations
	CreateCourse(ctx context.Context, userID uuid.UUID, req *CreateCourseRequest) (*model.Course, error)
	GetCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) (*model.Course, error)
	GetUserCourses(ctx context.Context, userID uuid.UUID) ([]model.Course, error)
	UpdateCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID, req *UpdateCourseRequest) (*model.Course, error)
	DeleteCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) error

	// Material operations
	CreateMaterial(ctx context.Context, userID uuid.UUID, req *CreateMaterialRequest) (*model.Material, error)
	GetMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID) (*model.Material, error)
	GetUserMaterials(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Material, error)
	GetCourseMaterials(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.Material, error)
	UpdateMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID, req *UpdateMaterialRequest) (*model.Material, error)
	DeleteMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID) error
	GeneratePresignURL(ctx context.Context, userID uuid.UUID, materialID uuid.UUID, action string) (*PresignResponse, error)
	UpdateMaterialProcessingStatus(ctx context.Context, materialID uuid.UUID, status string, chunks int, errMsg string) error

	// Quiz operations
	CreateQuiz(ctx context.Context, userID uuid.UUID, req *CreateQuizRequest) (*model.Quiz, error)
	GetQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.Quiz, error)
	GetUserQuizzes(ctx context.Context, userID uuid.UUID) ([]model.Quiz, error)
	GetCourseQuizzes(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.Quiz, error)
	UpdateQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID, req *UpdateQuizRequest) (*model.Quiz, error)
	DeleteQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) error

	// Quiz question operations
	AddQuizQuestion(ctx context.Context, userID uuid.UUID, quizID uuid.UUID, req *AddQuestionRequest) (*model.QuizQuestion, error)
	UpdateQuizQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID, req *UpdateQuestionRequest) (*model.QuizQuestion, error)
	DeleteQuizQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID) error

	// Flashcard operations
	CreateFlashcardDeck(ctx context.Context, userID uuid.UUID, req *CreateDeckRequest) (*model.FlashcardDeck, error)
	GetFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID) (*model.FlashcardDeck, error)
	GetUserFlashcardDecks(ctx context.Context, userID uuid.UUID) ([]model.FlashcardDeck, error)
	UpdateFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID, req *UpdateDeckRequest) (*model.FlashcardDeck, error)
	DeleteFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID) error

	// Flashcard card operations
	AddFlashcard(ctx context.Context, userID uuid.UUID, deckID uuid.UUID, req *AddFlashcardRequest) (*model.Flashcard, error)
	UpdateFlashcard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID, req *UpdateFlashcardRequest) (*model.Flashcard, error)
	DeleteFlashcard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) error
}

// PresignResponse holds the generated presigned URL.
type PresignResponse struct {
	URL        string `json:"url"`
	MaterialID string `json:"material_id"`
	ExpiresAt  int64  `json:"expires_at"`
}

// contentService implements ContentService.
type contentService struct {
	courseRepo     repository.CourseRepository
	materialRepo   repository.MaterialRepository
	quizRepo       repository.QuizRepository
	flashcardRepo  repository.FlashcardRepository
	minioClient    *minio.Client
	minioBucket    string
	minioPublicURL string
}

// NewContentService creates a new content service.
func NewContentService(
	courseRepo repository.CourseRepository,
	materialRepo repository.MaterialRepository,
	quizRepo repository.QuizRepository,
	flashcardRepo repository.FlashcardRepository,
	minioClient *minio.Client,
	minioBucket string,
	minioPublicURL string,
) ContentService {
	return &contentService{
		courseRepo:     courseRepo,
		materialRepo:   materialRepo,
		quizRepo:       quizRepo,
		flashcardRepo:  flashcardRepo,
		minioClient:    minioClient,
		minioBucket:    minioBucket,
		minioPublicURL: minioPublicURL,
	}
}

// Request/Response types

type CreateCourseRequest struct {
	InstitutionID    string `json:"institution_id,omitempty"`
	Code             string `json:"code,omitempty"`
	CourseOfferingID string `json:"course_offering_id,omitempty"`
	Name             string `json:"name" validate:"required,max=255"`
	Description      string `json:"description"`
	Color            string `json:"color"`
	Semester         string `json:"semester"`
	Year             int    `json:"year"`
	CreditUnits      int    `json:"credit_units,omitempty"`
	Syllabus         string `json:"syllabus,omitempty"`
}

type UpdateCourseRequest struct {
	InstitutionID    string `json:"institution_id,omitempty"`
	Code             string `json:"code,omitempty"`
	CourseOfferingID string `json:"course_offering_id,omitempty"`
	Name             string `json:"name,omitempty" validate:"omitempty,max=255"`
	Description      string `json:"description,omitempty"`
	Color            string `json:"color,omitempty"`
	Semester         string `json:"semester,omitempty"`
	Year             int    `json:"year,omitempty"`
	CreditUnits      int    `json:"credit_units,omitempty"`
	Syllabus         string `json:"syllabus,omitempty"`
}

type CreateMaterialRequest struct {
	CourseID            *uuid.UUID `json:"course_id,omitempty"`
	CourseOfferingID    string     `json:"course_offering_id,omitempty"`
	Title               string     `json:"title" validate:"required,max=255"`
	FileURL             string     `json:"file_url,omitempty"`
	FileSize            int64      `json:"file_size,omitempty"`
	MimeType            string     `json:"mime_type,omitempty"`
	Sha256Checksum      string     `json:"sha256_checksum,omitempty"`
	Version             int        `json:"version,omitempty"`
	TrackingID          string     `json:"tracking_id,omitempty"`
	DurationSeconds     int        `json:"duration_seconds,omitempty"`
	TranscriptionStatus string     `json:"transcription_status,omitempty"`
	TranscriptionText   string     `json:"transcription_text,omitempty"`
	AudioURL            string     `json:"audio_url,omitempty"`
}

type UpdateMaterialRequest struct {
	CourseID            *uuid.UUID `json:"course_id,omitempty"`
	CourseOfferingID    string     `json:"course_offering_id,omitempty"`
	Title               string     `json:"title,omitempty" validate:"omitempty,max=255"`
	FileURL             string     `json:"file_url,omitempty"`
	FileSize            int64      `json:"file_size,omitempty"`
	MimeType            string     `json:"mime_type,omitempty"`
	Sha256Checksum      string     `json:"sha256_checksum,omitempty"`
	Version             int        `json:"version,omitempty"`
	TrackingID          string     `json:"tracking_id,omitempty"`
	DurationSeconds     int        `json:"duration_seconds,omitempty"`
	TranscriptionStatus string     `json:"transcription_status,omitempty"`
	TranscriptionText   string     `json:"transcription_text,omitempty"`
	AudioURL            string     `json:"audio_url,omitempty"`
}

type CreateQuizRequest struct {
	CourseID         *uuid.UUID            `json:"course_id,omitempty"`
	MaterialID       *uuid.UUID            `json:"material_id,omitempty"`
	Title            string                `json:"title" validate:"required,max=255"`
	Description      string                `json:"description,omitempty"`
	TimeLimitMinutes int                   `json:"time_limit_minutes,omitempty"`
	Difficulty       model.DifficultyLevel `json:"difficulty,omitempty"`
	ShuffleQuestions bool                  `json:"shuffle_questions,omitempty"`
	Questions        []AddQuestionRequest  `json:"questions,omitempty"`
}

type UpdateQuizRequest struct {
	CourseID         *uuid.UUID            `json:"course_id,omitempty"`
	MaterialID       *uuid.UUID            `json:"material_id,omitempty"`
	Title            string                `json:"title,omitempty" validate:"omitempty,max=255"`
	Description      string                `json:"description,omitempty"`
	TimeLimitMinutes int                   `json:"time_limit_minutes,omitempty"`
	Difficulty       model.DifficultyLevel `json:"difficulty,omitempty"`
	ShuffleQuestions *bool                 `json:"shuffle_questions,omitempty"`
}

type AddQuestionRequest struct {
	QuestionText  string                `json:"question_text" validate:"required"`
	QuestionType  model.QuestionType    `json:"question_type" validate:"required"`
	Options       model.QuizOptions     `json:"options,omitempty"`
	CorrectAnswer string                `json:"correct_answer,omitempty"`
	Explanation   string                `json:"explanation,omitempty"`
	Points        int                   `json:"points,omitempty"`
	OrderIndex    int                   `json:"order_index"`
	Difficulty    model.DifficultyLevel `json:"difficulty,omitempty"`
}

type UpdateQuestionRequest struct {
	QuestionText  string                `json:"question_text,omitempty"`
	QuestionType  model.QuestionType    `json:"question_type,omitempty"`
	Options       model.QuizOptions     `json:"options,omitempty"`
	CorrectAnswer string                `json:"correct_answer,omitempty"`
	Explanation   string                `json:"explanation,omitempty"`
	Points        int                   `json:"points,omitempty"`
	OrderIndex    int                   `json:"order_index,omitempty"`
	Difficulty    model.DifficultyLevel `json:"difficulty,omitempty"`
}

type CreateDeckRequest struct {
	CourseID    *uuid.UUID            `json:"course_id,omitempty"`
	MaterialID  *uuid.UUID            `json:"material_id,omitempty"`
	Title       string                `json:"title" validate:"required,max=255"`
	Description string                `json:"description,omitempty"`
	Cards       []AddFlashcardRequest `json:"cards,omitempty"`
}

type UpdateDeckRequest struct {
	CourseID    *uuid.UUID `json:"course_id,omitempty"`
	MaterialID  *uuid.UUID `json:"material_id,omitempty"`
	Title       string     `json:"title,omitempty" validate:"omitempty,max=255"`
	Description string     `json:"description,omitempty"`
}

type AddFlashcardRequest struct {
	FrontText  string                `json:"front_text" validate:"required"`
	BackText   string                `json:"back_text" validate:"required"`
	Difficulty model.DifficultyLevel `json:"difficulty,omitempty"`
	OrderIndex int                   `json:"order_index"`
}

type UpdateFlashcardRequest struct {
	FrontText  string                `json:"front_text,omitempty"`
	BackText   string                `json:"back_text,omitempty"`
	Difficulty model.DifficultyLevel `json:"difficulty,omitempty"`
	OrderIndex int                   `json:"order_index,omitempty"`
}

// ==================== Course Operations ====================

func (s *contentService) CreateCourse(ctx context.Context, userID uuid.UUID, req *CreateCourseRequest) (*model.Course, error) {
	creditUnits := req.CreditUnits
	if creditUnits <= 0 {
		creditUnits = 3
	}
	course := &model.Course{
		UserID:           userID,
		InstitutionID:    req.InstitutionID,
		Code:             req.Code,
		CourseOfferingID: req.CourseOfferingID,
		Name:             req.Name,
		Description:      req.Description,
		Color:            req.Color,
		Semester:         req.Semester,
		Year:             req.Year,
		CreditUnits:      creditUnits,
		Syllabus:         req.Syllabus,
	}
	if course.Color == "" {
		course.Color = "#3B82F6"
	}

	if err := s.courseRepo.Create(ctx, course); err != nil {
		return nil, fmt.Errorf("failed to create course: %w", err)
	}
	return course, nil
}

func (s *contentService) GetCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) (*model.Course, error) {
	course, err := s.courseRepo.GetByID(ctx, courseID)
	if err != nil {
		return nil, ErrCourseNotFound
	}
	if course.UserID != userID {
		return nil, ErrUnauthorized
	}
	return course, nil
}

func (s *contentService) GetUserCourses(ctx context.Context, userID uuid.UUID) ([]model.Course, error) {
	return s.courseRepo.GetByUserID(ctx, userID)
}

func (s *contentService) UpdateCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID, req *UpdateCourseRequest) (*model.Course, error) {
	course, err := s.GetCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}

	if req.InstitutionID != "" {
		course.InstitutionID = req.InstitutionID
	}
	if req.Code != "" {
		course.Code = req.Code
	}
	if req.CourseOfferingID != "" {
		course.CourseOfferingID = req.CourseOfferingID
	}
	if req.Name != "" {
		course.Name = req.Name
	}
	if req.Description != "" {
		course.Description = req.Description
	}
	if req.Color != "" {
		course.Color = req.Color
	}
	if req.Semester != "" {
		course.Semester = req.Semester
	}
	if req.Year != 0 {
		course.Year = req.Year
	}
	if req.CreditUnits > 0 {
		course.CreditUnits = req.CreditUnits
	}
	if req.Syllabus != "" {
		course.Syllabus = req.Syllabus
	}

	if err := s.courseRepo.Update(ctx, course); err != nil {
		return nil, fmt.Errorf("failed to update course: %w", err)
	}
	return course, nil
}

func (s *contentService) DeleteCourse(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) error {
	_, err := s.GetCourse(ctx, userID, courseID)
	if err != nil {
		return err
	}
	return s.courseRepo.Delete(ctx, courseID)
}

// isAudioFile checks whether an audio URL, file URL, MIME type, or title represents audio content.
func isAudioFile(audioURL, fileURL, mimeType, title string) bool {
	if audioURL != "" {
		return true
	}
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mime, "audio/") {
		return true
	}
	audioExts := []string{".mp3", ".wav", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".wma", ".weba"}
	checkURL := strings.ToLower(fileURL)
	for _, ext := range audioExts {
		if strings.HasSuffix(checkURL, ext) {
			return true
		}
	}
	checkTitle := strings.ToLower(title)
	for _, ext := range audioExts {
		if strings.HasSuffix(checkTitle, ext) {
			return true
		}
	}
	return false
}

// ==================== Material Operations ====================

func (s *contentService) CreateMaterial(ctx context.Context, userID uuid.UUID, req *CreateMaterialRequest) (*model.Material, error) {
	// Verify course exists and belongs to user
	if req.CourseID != nil {
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
	}

	version := req.Version
	if version <= 0 {
		version = 1
	}

	trackingID := req.TrackingID
	transcriptionStatus := req.TranscriptionStatus
	if transcriptionStatus == "" {
		transcriptionStatus = "none"
	}

	// Asynchronous audio upload tracking: when AudioURL or audio file is provided,
	// generate a unique tracking_id if empty, and set transcription_status = "pending" for background processing.
	if isAudioFile(req.AudioURL, req.FileURL, req.MimeType, req.Title) {
		if trackingID == "" {
			trackingID = fmt.Sprintf("trk_%s", uuid.New().String())
		}
		transcriptionStatus = "pending"
	}

	material := &model.Material{
		UserID:              userID,
		CourseID:            req.CourseID,
		CourseOfferingID:    req.CourseOfferingID,
		Title:               req.Title,
		FileURL:             req.FileURL,
		FileSize:            req.FileSize,
		MimeType:            req.MimeType,
		Sha256Checksum:      req.Sha256Checksum,
		Version:             version,
		TrackingID:          trackingID,
		DurationSeconds:     req.DurationSeconds,
		ProcessingStatus:    model.ProcessingStatusPending,
		TranscriptionStatus: transcriptionStatus,
		TranscriptionText:   req.TranscriptionText,
		AudioURL:            req.AudioURL,
	}

	if err := s.materialRepo.Create(ctx, material); err != nil {
		return nil, fmt.Errorf("failed to create material: %w", err)
	}
	return material, nil
}

func (s *contentService) GetMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID) (*model.Material, error) {
	material, err := s.materialRepo.GetByID(ctx, materialID)
	if err != nil {
		return nil, ErrMaterialNotFound
	}
	if material.UserID != userID {
		return nil, ErrUnauthorized
	}
	return material, nil
}

func (s *contentService) GetUserMaterials(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Material, error) {
	return s.materialRepo.GetByUserID(ctx, userID, limit, offset)
}

func (s *contentService) GetCourseMaterials(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.Material, error) {
	// Verify course ownership
	_, err := s.GetCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	return s.materialRepo.GetByCourseID(ctx, courseID)
}

func (s *contentService) UpdateMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID, req *UpdateMaterialRequest) (*model.Material, error) {
	material, err := s.GetMaterial(ctx, userID, materialID)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		material.Title = req.Title
	}
	if req.CourseID != nil {
		// Verify course exists and belongs to user
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
		material.CourseID = req.CourseID
	}
	if req.CourseOfferingID != "" {
		material.CourseOfferingID = req.CourseOfferingID
	}
	if req.FileURL != "" {
		material.FileURL = req.FileURL
	}
	if req.FileSize != 0 {
		material.FileSize = req.FileSize
	}
	if req.MimeType != "" {
		material.MimeType = req.MimeType
	}
	if req.Sha256Checksum != "" {
		material.Sha256Checksum = req.Sha256Checksum
	}
	if req.Version > 0 {
		material.Version = req.Version
	}
	if req.DurationSeconds > 0 {
		material.DurationSeconds = req.DurationSeconds
	}
	if req.TrackingID != "" {
		material.TrackingID = req.TrackingID
	}
	if req.AudioURL != "" {
		material.AudioURL = req.AudioURL
	}
	if req.TranscriptionText != "" {
		material.TranscriptionText = req.TranscriptionText
	}
	if req.TranscriptionStatus != "" {
		material.TranscriptionStatus = req.TranscriptionStatus
	}

	// Asynchronous audio upload tracking: when AudioURL or audio file is provided,
	// generate a unique tracking_id if empty, and set transcription_status = "pending" for background processing.
	audioProvided := req.AudioURL != "" ||
		(req.MimeType != "" && strings.HasPrefix(strings.ToLower(req.MimeType), "audio/")) ||
		(req.FileURL != "" && isAudioFile("", req.FileURL, "", ""))
	if audioProvided {
		if material.TrackingID == "" {
			material.TrackingID = fmt.Sprintf("trk_%s", uuid.New().String())
		}
		if req.TranscriptionStatus == "" {
			material.TranscriptionStatus = "pending"
		}
	}

	if err := s.materialRepo.Update(ctx, material); err != nil {
		return nil, fmt.Errorf("failed to update material: %w", err)
	}
	return material, nil
}

func (s *contentService) DeleteMaterial(ctx context.Context, userID uuid.UUID, materialID uuid.UUID) error {
	_, err := s.GetMaterial(ctx, userID, materialID)
	if err != nil {
		return err
	}
	return s.materialRepo.Delete(ctx, materialID)
}

// UpdateMaterialProcessingStatus updates the processing status and summary for a material.
func (s *contentService) UpdateMaterialProcessingStatus(ctx context.Context, materialID uuid.UUID, status string, chunks int, errMsg string) error {
	var dbStatus model.ProcessingStatus
	switch status {
	case "completed":
		dbStatus = model.ProcessingStatusCompleted
	case "failed":
		dbStatus = model.ProcessingStatusFailed
	case "processing":
		dbStatus = model.ProcessingStatusProcessing
	default:
		dbStatus = model.ProcessingStatusPending
	}

	var summary string
	if dbStatus == model.ProcessingStatusCompleted {
		summary = fmt.Sprintf("Document processed successfully! Created %d chunks.", chunks)
	} else if dbStatus == model.ProcessingStatusFailed {
		if errMsg != "" {
			summary = fmt.Sprintf("Processing failed: %s", errMsg)
		} else {
			summary = "Processing failed with an unknown error."
		}
	}

	return s.materialRepo.UpdateProcessingStatus(ctx, materialID, dbStatus, summary)
}

// GeneratePresignURL generates a presigned URL for uploading a file to MinIO.
func (s *contentService) GeneratePresignURL(ctx context.Context, userID uuid.UUID, materialID uuid.UUID, action string) (*PresignResponse, error) {
	// Verify the material exists and belongs to the user
	_, err := s.GetMaterial(ctx, userID, materialID)
	if err != nil {
		return nil, err
	}

	if s.minioClient == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}

	// Build object name: materials/{user_id}/{material_id}
	objectName := fmt.Sprintf("materials/%s/%s", userID.String(), materialID.String())

	// Generate presigned PUT URL (valid for 15 minutes)
	expiry := 15 * time.Minute
	presignedURL, err := s.minioClient.PresignedPutObject(ctx, s.minioBucket, objectName, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	urlString := presignedURL.String()
	if s.minioPublicURL != "" {
		publicURL, err := url.Parse(s.minioPublicURL)
		if err == nil {
			presignedURL.Scheme = publicURL.Scheme
			presignedURL.Host = publicURL.Host
			urlString = presignedURL.String()
		}
	}

	return &PresignResponse{
		URL:        urlString,
		MaterialID: materialID.String(),
		ExpiresAt:  time.Now().Add(expiry).Unix(),
	}, nil
}

// ==================== Quiz Operations ====================

func (s *contentService) CreateQuiz(ctx context.Context, userID uuid.UUID, req *CreateQuizRequest) (*model.Quiz, error) {
	// Verify course exists and belongs to user
	if req.CourseID != nil {
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
	}

	// Verify material exists and belongs to user
	if req.MaterialID != nil {
		if _, err := s.GetMaterial(ctx, userID, *req.MaterialID); err != nil {
			return nil, err
		}
	}

	quiz := &model.Quiz{
		UserID:           userID,
		CourseID:         req.CourseID,
		MaterialID:       req.MaterialID,
		Title:            req.Title,
		Description:      req.Description,
		TimeLimitMinutes: req.TimeLimitMinutes,
		Difficulty:       req.Difficulty,
		ShuffleQuestions: req.ShuffleQuestions,
	}

	if err := s.quizRepo.Create(ctx, quiz); err != nil {
		return nil, fmt.Errorf("failed to create quiz: %w", err)
	}

	// Add questions if provided
	for i, qReq := range req.Questions {
		// Generate IDs for options if not provided
		options := make(model.QuizOptions, len(qReq.Options))
		for j, opt := range qReq.Options {
			if opt.ID == "" {
				opt.ID = uuid.New().String()
			}
			opt.OrderIndex = j
			options[j] = opt
		}

		question := &model.QuizQuestion{
			QuizID:        quiz.ID,
			QuestionText:  qReq.QuestionText,
			QuestionType:  qReq.QuestionType,
			Options:       options,
			CorrectAnswer: qReq.CorrectAnswer,
			Explanation:   qReq.Explanation,
			Points:        qReq.Points,
			OrderIndex:    i,
			Difficulty:    qReq.Difficulty,
		}
		if question.Points == 0 {
			question.Points = 1
		}
		if err := s.quizRepo.CreateQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("failed to create question: %w", err)
		}
	}

	// Reload quiz with questions
	return s.quizRepo.GetByID(ctx, quiz.ID)
}

func (s *contentService) GetQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.Quiz, error) {
	quiz, err := s.quizRepo.GetByID(ctx, quizID)
	if err != nil {
		return nil, ErrQuizNotFound
	}
	if quiz.UserID != userID {
		return nil, ErrUnauthorized
	}
	return quiz, nil
}

func (s *contentService) GetUserQuizzes(ctx context.Context, userID uuid.UUID) ([]model.Quiz, error) {
	return s.quizRepo.GetByUserID(ctx, userID)
}

func (s *contentService) GetCourseQuizzes(ctx context.Context, userID uuid.UUID, courseID uuid.UUID) ([]model.Quiz, error) {
	// Verify course ownership
	_, err := s.GetCourse(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	return s.quizRepo.GetByCourseID(ctx, courseID)
}

func (s *contentService) UpdateQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID, req *UpdateQuizRequest) (*model.Quiz, error) {
	quiz, err := s.GetQuiz(ctx, userID, quizID)
	if err != nil {
		return nil, err
	}

	if req.CourseID != nil {
		// Verify course exists and belongs to user
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
		quiz.CourseID = req.CourseID
	}
	if req.MaterialID != nil {
		// Verify material exists and belongs to user
		if _, err := s.GetMaterial(ctx, userID, *req.MaterialID); err != nil {
			return nil, err
		}
		quiz.MaterialID = req.MaterialID
	}
	if req.Title != "" {
		quiz.Title = req.Title
	}
	if req.Description != "" {
		quiz.Description = req.Description
	}
	if req.TimeLimitMinutes != 0 {
		quiz.TimeLimitMinutes = req.TimeLimitMinutes
	}
	if req.Difficulty != "" {
		quiz.Difficulty = req.Difficulty
	}
	if req.ShuffleQuestions != nil {
		quiz.ShuffleQuestions = *req.ShuffleQuestions
	}

	if err := s.quizRepo.Update(ctx, quiz); err != nil {
		return nil, fmt.Errorf("failed to update quiz: %w", err)
	}
	return quiz, nil
}

func (s *contentService) DeleteQuiz(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) error {
	_, err := s.GetQuiz(ctx, userID, quizID)
	if err != nil {
		return err
	}
	return s.quizRepo.Delete(ctx, quizID)
}

func (s *contentService) AddQuizQuestion(ctx context.Context, userID uuid.UUID, quizID uuid.UUID, req *AddQuestionRequest) (*model.QuizQuestion, error) {
	// Verify quiz ownership
	_, err := s.GetQuiz(ctx, userID, quizID)
	if err != nil {
		return nil, err
	}

	// Generate IDs for options if not provided
	options := make(model.QuizOptions, len(req.Options))
	for j, opt := range req.Options {
		if opt.ID == "" {
			opt.ID = uuid.New().String()
		}
		opt.OrderIndex = j
		options[j] = opt
	}

	question := &model.QuizQuestion{
		QuizID:        quizID,
		QuestionText:  req.QuestionText,
		QuestionType:  req.QuestionType,
		Options:       options,
		CorrectAnswer: req.CorrectAnswer,
		Explanation:   req.Explanation,
		Points:        req.Points,
		OrderIndex:    req.OrderIndex,
		Difficulty:    req.Difficulty,
	}
	if question.Points == 0 {
		question.Points = 1
	}

	if err := s.quizRepo.CreateQuestion(ctx, question); err != nil {
		return nil, fmt.Errorf("failed to create question: %w", err)
	}
	return question, nil
}

func (s *contentService) UpdateQuizQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID, req *UpdateQuestionRequest) (*model.QuizQuestion, error) {
	// Get question by ID
	targetQuestion, err := s.quizRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, ErrQuestionNotFound
	}

	// Verify quiz ownership
	_, err = s.GetQuiz(ctx, userID, targetQuestion.QuizID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.QuestionText != "" {
		targetQuestion.QuestionText = req.QuestionText
	}
	if req.QuestionType != "" {
		targetQuestion.QuestionType = req.QuestionType
	}
	if req.Options != nil {
		// Generate IDs for new options
		options := make(model.QuizOptions, len(req.Options))
		for j, opt := range req.Options {
			if opt.ID == "" {
				opt.ID = uuid.New().String()
			}
			opt.OrderIndex = j
			options[j] = opt
		}
		targetQuestion.Options = options
	}
	if req.CorrectAnswer != "" {
		targetQuestion.CorrectAnswer = req.CorrectAnswer
	}
	if req.Explanation != "" {
		targetQuestion.Explanation = req.Explanation
	}
	if req.Points != 0 {
		targetQuestion.Points = req.Points
	}
	if req.OrderIndex != 0 {
		targetQuestion.OrderIndex = req.OrderIndex
	}
	if req.Difficulty != "" {
		targetQuestion.Difficulty = req.Difficulty
	}

	if err := s.quizRepo.UpdateQuestion(ctx, targetQuestion); err != nil {
		return nil, fmt.Errorf("failed to update question: %w", err)
	}
	return targetQuestion, nil
}

func (s *contentService) DeleteQuizQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID) error {
	// Get question by ID to find quiz ID
	targetQuestion, err := s.quizRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return ErrQuestionNotFound
	}

	// Verify quiz ownership
	_, err = s.GetQuiz(ctx, userID, targetQuestion.QuizID)
	if err != nil {
		return err
	}

	return s.quizRepo.DeleteQuestion(ctx, questionID)
}

// ==================== Flashcard Operations ====================

func (s *contentService) CreateFlashcardDeck(ctx context.Context, userID uuid.UUID, req *CreateDeckRequest) (*model.FlashcardDeck, error) {
	// Verify course exists and belongs to user
	if req.CourseID != nil {
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
	}

	// Verify material exists and belongs to user
	if req.MaterialID != nil {
		if _, err := s.GetMaterial(ctx, userID, *req.MaterialID); err != nil {
			return nil, err
		}
	}

	deck := &model.FlashcardDeck{
		UserID:      userID,
		CourseID:    req.CourseID,
		MaterialID:  req.MaterialID,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := s.flashcardRepo.CreateDeck(ctx, deck); err != nil {
		return nil, fmt.Errorf("failed to create deck: %w", err)
	}

	// Add cards if provided
	for i, cReq := range req.Cards {
		card := &model.Flashcard{
			DeckID:     deck.ID,
			FrontText:  cReq.FrontText,
			BackText:   cReq.BackText,
			Difficulty: cReq.Difficulty,
			OrderIndex: i,
		}
		if err := s.flashcardRepo.CreateCard(ctx, card); err != nil {
			return nil, fmt.Errorf("failed to create card: %w", err)
		}
	}

	// Reload deck with cards
	return s.flashcardRepo.GetDeckByID(ctx, deck.ID)
}

func (s *contentService) GetFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID) (*model.FlashcardDeck, error) {
	deck, err := s.flashcardRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return nil, ErrFlashcardNotFound
	}
	if deck.UserID != userID {
		return nil, ErrUnauthorized
	}
	return deck, nil
}

func (s *contentService) GetUserFlashcardDecks(ctx context.Context, userID uuid.UUID) ([]model.FlashcardDeck, error) {
	return s.flashcardRepo.GetDecksByUserID(ctx, userID)
}

func (s *contentService) UpdateFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID, req *UpdateDeckRequest) (*model.FlashcardDeck, error) {
	deck, err := s.GetFlashcardDeck(ctx, userID, deckID)
	if err != nil {
		return nil, err
	}

	if req.CourseID != nil {
		// Verify course exists and belongs to user
		if _, err := s.GetCourse(ctx, userID, *req.CourseID); err != nil {
			return nil, err
		}
		deck.CourseID = req.CourseID
	}
	if req.MaterialID != nil {
		// Verify material exists and belongs to user
		if _, err := s.GetMaterial(ctx, userID, *req.MaterialID); err != nil {
			return nil, err
		}
		deck.MaterialID = req.MaterialID
	}
	if req.Title != "" {
		deck.Title = req.Title
	}
	if req.Description != "" {
		deck.Description = req.Description
	}

	if err := s.flashcardRepo.UpdateDeck(ctx, deck); err != nil {
		return nil, fmt.Errorf("failed to update deck: %w", err)
	}
	return deck, nil
}

func (s *contentService) DeleteFlashcardDeck(ctx context.Context, userID uuid.UUID, deckID uuid.UUID) error {
	_, err := s.GetFlashcardDeck(ctx, userID, deckID)
	if err != nil {
		return err
	}
	return s.flashcardRepo.DeleteDeck(ctx, deckID)
}

func (s *contentService) AddFlashcard(ctx context.Context, userID uuid.UUID, deckID uuid.UUID, req *AddFlashcardRequest) (*model.Flashcard, error) {
	// Verify deck ownership
	_, err := s.GetFlashcardDeck(ctx, userID, deckID)
	if err != nil {
		return nil, err
	}

	card := &model.Flashcard{
		DeckID:     deckID,
		FrontText:  req.FrontText,
		BackText:   req.BackText,
		Difficulty: req.Difficulty,
		OrderIndex: req.OrderIndex,
	}

	if err := s.flashcardRepo.CreateCard(ctx, card); err != nil {
		return nil, fmt.Errorf("failed to create card: %w", err)
	}
	return card, nil
}

func (s *contentService) UpdateFlashcard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID, req *UpdateFlashcardRequest) (*model.Flashcard, error) {
	// Get card
	card, err := s.flashcardRepo.GetCardByID(ctx, cardID)
	if err != nil {
		return nil, ErrFlashcardNotFound
	}

	// Verify deck ownership
	_, err = s.GetFlashcardDeck(ctx, userID, card.DeckID)
	if err != nil {
		return nil, err
	}

	if req.FrontText != "" {
		card.FrontText = req.FrontText
	}
	if req.BackText != "" {
		card.BackText = req.BackText
	}
	if req.Difficulty != "" {
		card.Difficulty = req.Difficulty
	}
	if req.OrderIndex != 0 {
		card.OrderIndex = req.OrderIndex
	}

	if err := s.flashcardRepo.UpdateCard(ctx, card); err != nil {
		return nil, fmt.Errorf("failed to update card: %w", err)
	}
	return card, nil
}

func (s *contentService) DeleteFlashcard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) error {
	// Get card
	card, err := s.flashcardRepo.GetCardByID(ctx, cardID)
	if err != nil {
		return ErrFlashcardNotFound
	}

	// Verify deck ownership
	_, err = s.GetFlashcardDeck(ctx, userID, card.DeckID)
	if err != nil {
		return err
	}

	return s.flashcardRepo.DeleteCard(ctx, cardID)
}
