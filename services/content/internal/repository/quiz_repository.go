package repository

import (
	"context"

	"github.com/google/uuid"

	"zuri/services/content/internal/model"
	"zuri/shared/pkg/database"
)

// QuizRepository defines the interface for quiz data access.
type QuizRepository interface {
	Create(ctx context.Context, quiz *model.Quiz) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Quiz, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Quiz, error)
	GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Quiz, error)
	GetByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.Quiz, error)
	Update(ctx context.Context, quiz *model.Quiz) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Question methods
	CreateQuestion(ctx context.Context, question *model.QuizQuestion) error
	GetQuestionsByQuizID(ctx context.Context, quizID uuid.UUID) ([]model.QuizQuestion, error)
	GetQuestionByID(ctx context.Context, id uuid.UUID) (*model.QuizQuestion, error)
	UpdateQuestion(ctx context.Context, question *model.QuizQuestion) error
	DeleteQuestion(ctx context.Context, id uuid.UUID) error
}

// quizRepository implements QuizRepository.
type quizRepository struct {
	db *database.DB
}

// NewQuizRepository creates a new quiz repository.
func NewQuizRepository(db *database.DB) QuizRepository {
	return &quizRepository{db: db}
}

// Create creates a new quiz with questions.
func (r *quizRepository) Create(ctx context.Context, quiz *model.Quiz) error {
	return r.db.DB.WithContext(ctx).Create(quiz).Error
}

// GetByID retrieves a quiz by ID with questions.
func (r *quizRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Quiz, error) {
	var quiz model.Quiz
	err := r.db.WithContext(ctx).
		Preload("Course").
		Preload("Material").
		Preload("Questions").
		First(&quiz, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &quiz, nil
}

// GetByUserID retrieves quizzes for a user.
func (r *quizRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Quiz, error) {
	var quizzes []model.Quiz
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&quizzes).Error
	return quizzes, err
}

// GetByCourseID retrieves quizzes for a course.
func (r *quizRepository) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Quiz, error) {
	var quizzes []model.Quiz
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Order("created_at DESC").
		Find(&quizzes).Error
	return quizzes, err
}

// GetByMaterialID retrieves quizzes for a material.
func (r *quizRepository) GetByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.Quiz, error) {
	var quizzes []model.Quiz
	err := r.db.WithContext(ctx).
		Where("material_id = ?", materialID).
		Order("created_at DESC").
		Find(&quizzes).Error
	return quizzes, err
}

// Update updates a quiz.
func (r *quizRepository) Update(ctx context.Context, quiz *model.Quiz) error {
	return r.db.DB.WithContext(ctx).Save(quiz).Error
}

// Delete soft-deletes a quiz.
func (r *quizRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.Quiz{}, "id = ?", id).Error
}

// CreateQuestion creates a new quiz question.
func (r *quizRepository) CreateQuestion(ctx context.Context, question *model.QuizQuestion) error {
	return r.db.DB.WithContext(ctx).Create(question).Error
}

// GetQuestionsByQuizID retrieves questions for a quiz.
func (r *quizRepository) GetQuestionsByQuizID(ctx context.Context, quizID uuid.UUID) ([]model.QuizQuestion, error) {
	var questions []model.QuizQuestion
	err := r.db.WithContext(ctx).
		Where("quiz_id = ?", quizID).
		Order("order_index").
		Find(&questions).Error
	return questions, err
}

// GetQuestionByID retrieves a single quiz question by ID.
func (r *quizRepository) GetQuestionByID(ctx context.Context, id uuid.UUID) (*model.QuizQuestion, error) {
	var question model.QuizQuestion
	err := r.db.WithContext(ctx).First(&question, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &question, nil
}

// UpdateQuestion updates a quiz question.
func (r *quizRepository) UpdateQuestion(ctx context.Context, question *model.QuizQuestion) error {
	return r.db.DB.WithContext(ctx).Save(question).Error
}

// DeleteQuestion deletes a quiz question.
func (r *quizRepository) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.QuizQuestion{}, "id = ?", id).Error
}
