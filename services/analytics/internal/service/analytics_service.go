// Package service provides business logic for analytics.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"zuri/services/analytics/internal/model"
	"zuri/services/analytics/internal/repository"
)

var (
	ErrAttemptNotFound    = errors.New("quiz attempt not found")
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrInvalidAnswer      = errors.New("invalid answer")
	ErrQuestionNotFound   = errors.New("question not found")
)

// AnalyticsService defines the analytics management interface.
type AnalyticsService interface {
	// Quiz attempt operations
	StartQuizAttempt(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.QuizAttempt, error)
	SubmitAnswer(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID, questionID uuid.UUID, answer string, timeTakenSeconds int) (*model.QuizAnswer, error)
	CompleteQuizAttempt(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID) (*model.QuizAttempt, error)
	GetAttempt(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID) (*model.QuizAttempt, error)
	GetUserAttempts(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.QuizAttempt, error)
	GetQuizAttempts(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) ([]model.QuizAttempt, error)
	GetBestQuizScore(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.QuizAttempt, error)
	
	// Study session operations
	RecordStudySession(ctx context.Context, userID uuid.UUID, req *RecordStudySessionRequest) error
	GetStudyStreak(ctx context.Context, userID uuid.UUID) (int, error)
	GetUserStudyStats(ctx context.Context, userID uuid.UUID) (*model.UserStudyStats, error)
	GetRecentStudySessions(ctx context.Context, userID uuid.UUID, days int) ([]model.StudySession, error)
	
	// Topic mastery operations
	UpdateTopicMastery(ctx context.Context, userID uuid.UUID, courseID *uuid.UUID, topic string, isCorrect bool) error
	GetTopicMastery(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error)
	GetTopicsForReview(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error)
	
	// Learning goal operations
	CreateLearningGoal(ctx context.Context, userID uuid.UUID, req *CreateLearningGoalRequest) (*model.LearningGoal, error)
	GetLearningGoals(ctx context.Context, userID uuid.UUID, includeCompleted bool) ([]model.LearningGoal, error)
	UpdateLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID, req *UpdateLearningGoalRequest) (*model.LearningGoal, error)
	CompleteLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID) error
	DeleteLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID) error
	
	// AI interaction tracking
	TrackAIInteraction(ctx context.Context, userID uuid.UUID, req *TrackAIInteractionRequest) error
	GetAIUsageStats(ctx context.Context, userID uuid.UUID, days int) (*repository.AIUsageStats, error)
}

// analyticsService implements AnalyticsService.
type analyticsService struct {
	attemptRepo   repository.QuizAttemptRepository
	sessionRepo   repository.StudySessionRepository
	masteryRepo   repository.TopicMasteryRepository
	aiRepo        repository.AIInteractionRepository
	goalRepo      repository.LearningGoalRepository
}

// NewAnalyticsService creates a new analytics service.
func NewAnalyticsService(
	attemptRepo repository.QuizAttemptRepository,
	sessionRepo repository.StudySessionRepository,
	masteryRepo repository.TopicMasteryRepository,
	aiRepo repository.AIInteractionRepository,
	goalRepo repository.LearningGoalRepository,
) AnalyticsService {
	return &analyticsService{
		attemptRepo:   attemptRepo,
		sessionRepo:   sessionRepo,
		masteryRepo:   masteryRepo,
		aiRepo:        aiRepo,
		goalRepo:      goalRepo,
	}
}

// Request/Response types

type RecordStudySessionRequest struct {
	SessionDate        time.Time `json:"session_date"`
	DurationMinutes    int       `json:"duration_minutes"`
	MaterialsReviewed  int       `json:"materials_reviewed"`
	QuizzesCompleted   int       `json:"quizzes_completed"`
	FlashcardsReviewed int       `json:"flashcards_reviewed"`
}

// FlexibleDate handles both "2026-06-19" and "2026-06-19T00:00:00Z" formats.
type FlexibleDate struct {
	time.Time
}

func (fd *FlexibleDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	// Try RFC 3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		fd.Time = t
		return nil
	}
	// Fall back to date-only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		fd.Time = t
		return nil
	}
	return fmt.Errorf("cannot parse date: %s", s)
}

func (fd *FlexibleDate) ToTimePtr() *time.Time {
	if fd == nil || fd.Time.IsZero() {
		return nil
	}
	t := fd.Time
	return &t
}

type CreateLearningGoalRequest struct {
	CourseID    *uuid.UUID      `json:"course_id,omitempty"`
	Title       string          `json:"title" validate:"required,max=255"`
	Description string          `json:"description,omitempty"`
	TargetDate  *FlexibleDate   `json:"target_date,omitempty"`
	TargetScore *int            `json:"target_score,omitempty"`
	GoalType    model.GoalType  `json:"goal_type,omitempty"`
}

type UpdateLearningGoalRequest struct {
	CourseID    *uuid.UUID      `json:"course_id,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	TargetDate  *FlexibleDate   `json:"target_date,omitempty"`
	TargetScore *int            `json:"target_score,omitempty"`
	GoalType    model.GoalType  `json:"goal_type,omitempty"`
}

type TrackAIInteractionRequest struct {
	InteractionType  model.AIInteractionType `json:"interaction_type" validate:"required"`
	MaterialID       *uuid.UUID              `json:"material_id,omitempty"`
	PromptTokens     int                     `json:"prompt_tokens"`
	CompletionTokens int                     `json:"completion_tokens"`
	TotalTokens      int                     `json:"total_tokens"`
	LatencyMs        int                     `json:"latency_ms"`
	Success          bool                    `json:"success"`
	ErrorMessage     string                  `json:"error_message,omitempty"`
	Model            string                  `json:"model,omitempty"`
}

// ==================== Quiz Attempt Operations ====================

func (s *analyticsService) StartQuizAttempt(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.QuizAttempt, error) {
	attempt := &model.QuizAttempt{
		UserID:    userID,
		QuizID:    quizID,
		StartedAt: time.Now(),
		Status:    model.AttemptStatusInProgress,
	}
	
	if err := s.attemptRepo.Create(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to create quiz attempt: %w", err)
	}
	
	// Record study session
	session := &model.StudySession{
		UserID:      userID,
		SessionDate: time.Now().Truncate(24 * time.Hour),
	}
	s.sessionRepo.CreateOrUpdate(ctx, session)
	
	return attempt, nil
}

func (s *analyticsService) SubmitAnswer(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID, questionID uuid.UUID, answer string, timeTakenSeconds int) (*model.QuizAnswer, error) {
	// Get attempt
	attempt, err := s.attemptRepo.GetByID(ctx, attemptID)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	
	if attempt.UserID != userID {
		return nil, ErrUnauthorized
	}
	
	if attempt.Status != model.AttemptStatusInProgress {
		return nil, errors.New("attempt is not in progress")
	}
	
	// Create answer (grading will be done when completing)
	quizAnswer := &model.QuizAnswer{
		AttemptID:        attemptID,
		QuestionID:       questionID,
		UserAnswer:       answer,
		TimeTakenSeconds: &timeTakenSeconds,
	}
	
	if err := s.attemptRepo.CreateAnswer(ctx, quizAnswer); err != nil {
		return nil, fmt.Errorf("failed to save answer: %w", err)
	}
	
	return quizAnswer, nil
}

func (s *analyticsService) CompleteQuizAttempt(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID) (*model.QuizAttempt, error) {
	// Get attempt with answers
	attempt, err := s.attemptRepo.GetByID(ctx, attemptID)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	
	if attempt.UserID != userID {
		return nil, ErrUnauthorized
	}
	
	// Note: In a real implementation, we would fetch questions from Content Service
	// For now, we'll calculate based on stored answers
	answers, err := s.attemptRepo.GetAnswersByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	
	// Auto-grade answers (simplified - would need question data from Content Service)
	var score, maxScore int
	for i := range answers {
		// Simple grading: assume 1 point per question for now
		// In reality, we'd compare with correct answer from question
		answers[i].PointsEarned = 1 // Placeholder
		score += answers[i].PointsEarned
		maxScore += 1
	}
	
	now := time.Now()
	timeTaken := int(now.Sub(attempt.StartedAt).Seconds())
	
	var percentage float64
	if maxScore > 0 {
		percentage = float64(score) / float64(maxScore) * 100
	}
	
	attempt.Status = model.AttemptStatusCompleted
	attempt.CompletedAt = &now
	attempt.Score = &score
	attempt.MaxScore = &maxScore
	attempt.Percentage = &percentage
	attempt.TimeTakenSeconds = &timeTaken
	
	if err := s.attemptRepo.Update(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to complete attempt: %w", err)
	}
	
	// Update study session
	session := &model.StudySession{
		UserID:           userID,
		SessionDate:      now.Truncate(24 * time.Hour),
		QuizzesCompleted: 1,
	}
	s.sessionRepo.CreateOrUpdate(ctx, session)
	
	return attempt, nil
}

func (s *analyticsService) GetAttempt(ctx context.Context, userID uuid.UUID, attemptID uuid.UUID) (*model.QuizAttempt, error) {
	attempt, err := s.attemptRepo.GetByID(ctx, attemptID)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	if attempt.UserID != userID {
		return nil, ErrUnauthorized
	}
	return attempt, nil
}

func (s *analyticsService) GetUserAttempts(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.QuizAttempt, error) {
	return s.attemptRepo.GetByUserID(ctx, userID, limit, offset)
}

func (s *analyticsService) GetQuizAttempts(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) ([]model.QuizAttempt, error) {
	return s.attemptRepo.GetByQuizID(ctx, quizID, userID)
}

func (s *analyticsService) GetBestQuizScore(ctx context.Context, userID uuid.UUID, quizID uuid.UUID) (*model.QuizAttempt, error) {
	return s.attemptRepo.GetBestAttempt(ctx, quizID, userID)
}

// ==================== Study Session Operations ====================

func (s *analyticsService) RecordStudySession(ctx context.Context, userID uuid.UUID, req *RecordStudySessionRequest) error {
	session := &model.StudySession{
		UserID:             userID,
		SessionDate:        req.SessionDate.Truncate(24 * time.Hour),
		DurationMinutes:    req.DurationMinutes,
		MaterialsReviewed:  req.MaterialsReviewed,
		QuizzesCompleted:   req.QuizzesCompleted,
		FlashcardsReviewed: req.FlashcardsReviewed,
	}
	
	return s.sessionRepo.CreateOrUpdate(ctx, session)
}

func (s *analyticsService) GetStudyStreak(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.sessionRepo.GetStudyStreak(ctx, userID)
}

func (s *analyticsService) GetUserStudyStats(ctx context.Context, userID uuid.UUID) (*model.UserStudyStats, error) {
	return s.sessionRepo.GetUserStats(ctx, userID)
}

func (s *analyticsService) GetRecentStudySessions(ctx context.Context, userID uuid.UUID, days int) ([]model.StudySession, error) {
	return s.sessionRepo.GetRecentSessions(ctx, userID, days)
}

// ==================== Topic Mastery Operations ====================

func (s *analyticsService) UpdateTopicMastery(ctx context.Context, userID uuid.UUID, courseID *uuid.UUID, topic string, isCorrect bool) error {
	return s.masteryRepo.UpdateMastery(ctx, userID, courseID, topic, isCorrect)
}

func (s *analyticsService) GetTopicMastery(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error) {
	return s.masteryRepo.GetByUserID(ctx, userID)
}

func (s *analyticsService) GetTopicsForReview(ctx context.Context, userID uuid.UUID) ([]model.TopicMastery, error) {
	return s.masteryRepo.GetTopicsForReview(ctx, userID, time.Now())
}

// ==================== Learning Goal Operations ====================

func (s *analyticsService) CreateLearningGoal(ctx context.Context, userID uuid.UUID, req *CreateLearningGoalRequest) (*model.LearningGoal, error) {
	goal := &model.LearningGoal{
		UserID:      userID,
		CourseID:    req.CourseID,
		Title:       req.Title,
		Description: req.Description,
		TargetDate:  req.TargetDate.ToTimePtr(),
		TargetScore: req.TargetScore,
		GoalType:    req.GoalType,
		IsCompleted: false,
	}
	
	if err := s.goalRepo.Create(ctx, goal); err != nil {
		return nil, fmt.Errorf("failed to create learning goal: %w", err)
	}
	
	return goal, nil
}

func (s *analyticsService) GetLearningGoals(ctx context.Context, userID uuid.UUID, includeCompleted bool) ([]model.LearningGoal, error) {
	goals, err := s.goalRepo.GetByUserID(ctx, userID, includeCompleted)
	if err != nil {
		return nil, err
	}
	
	// Enrich each goal with computed current_value based on goal type
	for i := range goals {
		if goals[i].IsCompleted {
			goals[i].CurrentValue = 0
			if goals[i].TargetScore != nil {
				goals[i].CurrentValue = *goals[i].TargetScore
			}
			continue
		}
		
		current, err := s.computeGoalProgress(ctx, userID, &goals[i])
		if err != nil {
			// Log but don't fail — return goal with zero progress
			continue
		}
		goals[i].CurrentValue = current
		
		// Auto-complete if target reached
		if goals[i].TargetScore != nil && current >= *goals[i].TargetScore {
			_ = s.goalRepo.MarkComplete(ctx, goals[i].ID)
			goals[i].IsCompleted = true
			goals[i].CurrentValue = *goals[i].TargetScore
		}
	}
	
	return goals, nil
}

func (s *analyticsService) computeGoalProgress(ctx context.Context, userID uuid.UUID, goal *model.LearningGoal) (int, error) {
	switch goal.GoalType {
	case model.GoalTypeStudyTime:
		stats, err := s.sessionRepo.GetUserStats(ctx, userID)
		if err != nil {
			return 0, err
		}
		return stats.TotalStudyMinutes, nil
		
	case model.GoalTypeQuizScore:
		attempts, err := s.attemptRepo.GetByUserID(ctx, userID, 1000, 0)
		if err != nil {
			return 0, err
		}
		best := 0
		for _, attempt := range attempts {
			if attempt.Status == model.AttemptStatusCompleted && attempt.Percentage != nil {
				pct := int(*attempt.Percentage)
				if pct > best {
					best = pct
				}
			}
		}
		return best, nil
		
	case model.GoalTypeStreak:
		streak, err := s.sessionRepo.GetStudyStreak(ctx, userID)
		if err != nil {
			return 0, err
		}
		return streak, nil
		
	case model.GoalTypeCourseCompletion:
		attempts, err := s.attemptRepo.GetByUserID(ctx, userID, 1000, 0)
		if err != nil {
			return 0, err
		}
		count := 0
		for _, attempt := range attempts {
			if attempt.Status == model.AttemptStatusCompleted {
				count++
			}
		}
		return count, nil
		
	default:
		return 0, nil
	}
}

func (s *analyticsService) UpdateLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID, req *UpdateLearningGoalRequest) (*model.LearningGoal, error) {
	goal, err := s.goalRepo.GetByID(ctx, goalID)
	if err != nil {
		return nil, errors.New("goal not found")
	}
	
	if goal.UserID != userID {
		return nil, ErrUnauthorized
	}
	
	if req.Title != "" {
		goal.Title = req.Title
	}
	if req.Description != "" {
		goal.Description = req.Description
	}
	if req.CourseID != nil {
		goal.CourseID = req.CourseID
	}
	if req.TargetDate != nil {
		goal.TargetDate = req.TargetDate.ToTimePtr()
	}
	if req.TargetScore != nil {
		goal.TargetScore = req.TargetScore
	}
	if req.GoalType != "" {
		goal.GoalType = req.GoalType
	}
	
	if err := s.goalRepo.Update(ctx, goal); err != nil {
		return nil, fmt.Errorf("failed to update learning goal: %w", err)
	}
	
	return goal, nil
}

func (s *analyticsService) CompleteLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID) error {
	// Get goal
	goal, err := s.goalRepo.GetByID(ctx, goalID)
	if err != nil {
		return errors.New("goal not found")
	}
	
	if goal.UserID != userID {
		return ErrUnauthorized
	}
	
	return s.goalRepo.MarkComplete(ctx, goalID)
}

func (s *analyticsService) DeleteLearningGoal(ctx context.Context, userID uuid.UUID, goalID uuid.UUID) error {
	goal, err := s.goalRepo.GetByID(ctx, goalID)
	if err != nil {
		return errors.New("goal not found")
	}
	
	if goal.UserID != userID {
		return ErrUnauthorized
	}
	
	return s.goalRepo.Delete(ctx, goalID)
}

// ==================== AI Interaction Operations ====================

func (s *analyticsService) TrackAIInteraction(ctx context.Context, userID uuid.UUID, req *TrackAIInteractionRequest) error {
	interaction := &model.AIInteraction{
		UserID:           userID,
		InteractionType:  req.InteractionType,
		MaterialID:       req.MaterialID,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		TotalTokens:      req.TotalTokens,
		LatencyMs:        &req.LatencyMs,
		Success:          req.Success,
		ErrorMessage:     &req.ErrorMessage,
		Model:            &req.Model,
	}
	
	if req.ErrorMessage == "" {
		interaction.ErrorMessage = nil
	}
	
	return s.aiRepo.Create(ctx, interaction)
}

func (s *analyticsService) GetAIUsageStats(ctx context.Context, userID uuid.UUID, days int) (*repository.AIUsageStats, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)
	return s.aiRepo.GetUsageStats(ctx, userID, startDate, endDate)
}

// ==================== Helper Functions ====================

// calculateSimilarity calculates string similarity using Levenshtein distance
func calculateSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	
	if a == b {
		return 1.0
	}
	
	distance := levenshteinDistance(a, b)
	maxLen := math.Max(float64(len(a)), float64(len(b)))
	
	if maxLen == 0 {
		return 1.0
	}
	
	return 1.0 - float64(distance)/maxLen
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	
	// Convert to rune slices for Unicode support
	ra := []rune(a)
	rb := []rune(b)
	
	// Create matrix
	matrix := make([][]int, len(ra)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(rb)+1)
	}
	
	// Initialize first row and column
	for i := 0; i <= len(ra); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(rb); j++ {
		matrix[0][j] = j
	}
	
	// Fill matrix
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost
			
			matrix[i][j] = min(deletion, min(insertion, substitution))
		}
	}
	
	return matrix[len(ra)][len(rb)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizeString removes extra spaces and converts to lowercase
func normalizeString(s string) string {
	var result []rune
	var lastSpace bool
	
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			if !lastSpace {
				result = append(result, ' ')
				lastSpace = true
			}
		} else {
			result = append(result, r)
			lastSpace = false
		}
	}
	
	return strings.TrimSpace(string(result))
}
