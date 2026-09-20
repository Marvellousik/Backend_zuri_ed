package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"zuri/services/analytics/internal/model"
	"zuri/shared/pkg/database"
)

// StudySessionRepository defines the interface for study session data access.
type StudySessionRepository interface {
	CreateOrUpdate(ctx context.Context, session *model.StudySession) error
	GetByUserIDAndDate(ctx context.Context, userID uuid.UUID, date time.Time) (*model.StudySession, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.StudySession, error)
	GetRecentSessions(ctx context.Context, userID uuid.UUID, days int) ([]model.StudySession, error)
	GetStudyStreak(ctx context.Context, userID uuid.UUID) (int, error)
	GetUserStats(ctx context.Context, userID uuid.UUID) (*model.UserStudyStats, error)
}

// studySessionRepository implements StudySessionRepository.
type studySessionRepository struct {
	db *database.DB
}

// NewStudySessionRepository creates a new study session repository.
func NewStudySessionRepository(db *database.DB) StudySessionRepository {
	return &studySessionRepository{db: db}
}

// CreateOrUpdate creates or updates a study session.
func (r *studySessionRepository) CreateOrUpdate(ctx context.Context, session *model.StudySession) error {
	// Try to find existing session for this date
	var existing model.StudySession
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND session_date = ?", session.UserID, session.SessionDate).
		First(&existing).Error
	
	if err == nil {
		// Update existing session
		existing.DurationMinutes += session.DurationMinutes
		existing.MaterialsReviewed += session.MaterialsReviewed
		existing.QuizzesCompleted += session.QuizzesCompleted
		existing.FlashcardsReviewed += session.FlashcardsReviewed
		return r.db.DB.WithContext(ctx).Save(&existing).Error
	}
	
	// Create new session
	return r.db.DB.WithContext(ctx).Create(session).Error
}

// GetByUserIDAndDate retrieves a study session for a specific date.
func (r *studySessionRepository) GetByUserIDAndDate(ctx context.Context, userID uuid.UUID, date time.Time) (*model.StudySession, error) {
	var session model.StudySession
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND session_date = ?", userID, date).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByUserID retrieves study sessions for a user.
func (r *studySessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]model.StudySession, error) {
	var sessions []model.StudySession
	query := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("session_date DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&sessions).Error
	return sessions, err
}

// GetRecentSessions retrieves sessions from the last N days.
func (r *studySessionRepository) GetRecentSessions(ctx context.Context, userID uuid.UUID, days int) ([]model.StudySession, error) {
	var sessions []model.StudySession
	cutoffDate := time.Now().AddDate(0, 0, -days)
	
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND session_date >= ?", userID, cutoffDate).
		Order("session_date DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetStudyStreak calculates the current study streak (consecutive days with activity).
func (r *studySessionRepository) GetStudyStreak(ctx context.Context, userID uuid.UUID) (int, error) {
	// Get all sessions ordered by date
	var sessions []model.StudySession
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("session_date DESC").
		Find(&sessions).Error
	if err != nil {
		return 0, err
	}
	
	if len(sessions) == 0 {
		return 0, nil
	}
	
	// Calculate streak
	streak := 0
	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	
	// Check if studied today or yesterday (streak can be active today or yesterday)
	mostRecentSession := sessions[0].SessionDate.Truncate(24 * time.Hour)
	if mostRecentSession.Before(yesterday) {
		return 0, nil // Streak broken
	}
	
	for i, session := range sessions {
		expectedDate := today.AddDate(0, 0, -i)
		sessionDate := session.SessionDate.Truncate(24 * time.Hour)
		
		if sessionDate.Equal(expectedDate) || sessionDate.Equal(expectedDate.AddDate(0, 0, 1)) {
			streak++
		} else {
			break
		}
	}
	
	return streak, nil
}

// GetUserStats retrieves aggregated study statistics for a user.
func (r *studySessionRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (*model.UserStudyStats, error) {
	var stats model.UserStudyStats
	stats.UserID = userID
	
	err := r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Count distinct study days
		var totalDays int64
		if err := tx.Model(&model.StudySession{}).
			Where("user_id = ?", userID).
			Select("COUNT(DISTINCT session_date)").
			Scan(&totalDays).Error; err != nil {
			return err
		}
		stats.TotalStudyDays = int(totalDays)
		
		// Sum duration
		var totalMinutes int64
		if err := tx.Model(&model.StudySession{}).
			Where("user_id = ?", userID).
			Select("COALESCE(SUM(duration_minutes), 0)").
			Scan(&totalMinutes).Error; err != nil {
			return err
		}
		stats.TotalStudyMinutes = int(totalMinutes)
		
		// Sum quizzes
		var totalQuizzes int64
		if err := tx.Model(&model.StudySession{}).
			Where("user_id = ?", userID).
			Select("COALESCE(SUM(quizzes_completed), 0)").
			Scan(&totalQuizzes).Error; err != nil {
			return err
		}
		stats.TotalQuizzesCompleted = int(totalQuizzes)
		
		// Sum materials
		var totalMaterials int64
		if err := tx.Model(&model.StudySession{}).
			Where("user_id = ?", userID).
			Select("COALESCE(SUM(materials_reviewed), 0)").
			Scan(&totalMaterials).Error; err != nil {
			return err
		}
		stats.TotalMaterialsReviewed = int(totalMaterials)
		
		// Get last study date
		var lastSession model.StudySession
		if err := tx.Where("user_id = ?", userID).
			Order("session_date DESC").
			First(&lastSession).Error; err == nil {
			stats.LastStudyDate = &lastSession.SessionDate
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return &stats, nil
}
