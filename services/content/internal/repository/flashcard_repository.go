package repository

import (
	"context"

	"github.com/google/uuid"

	"zuri/services/content/internal/model"
	"zuri/shared/pkg/database"
)

// FlashcardRepository defines the interface for flashcard data access.
type FlashcardRepository interface {
	// Deck methods
	CreateDeck(ctx context.Context, deck *model.FlashcardDeck) error
	GetDeckByID(ctx context.Context, id uuid.UUID) (*model.FlashcardDeck, error)
	GetDecksByUserID(ctx context.Context, userID uuid.UUID) ([]model.FlashcardDeck, error)
	GetDecksByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.FlashcardDeck, error)
	GetDecksByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.FlashcardDeck, error)
	UpdateDeck(ctx context.Context, deck *model.FlashcardDeck) error
	DeleteDeck(ctx context.Context, id uuid.UUID) error

	// Card methods
	CreateCard(ctx context.Context, card *model.Flashcard) error
	GetCardsByDeckID(ctx context.Context, deckID uuid.UUID) ([]model.Flashcard, error)
	GetCardByID(ctx context.Context, id uuid.UUID) (*model.Flashcard, error)
	UpdateCard(ctx context.Context, card *model.Flashcard) error
	DeleteCard(ctx context.Context, id uuid.UUID) error
}

// flashcardRepository implements FlashcardRepository.
type flashcardRepository struct {
	db *database.DB
}

// NewFlashcardRepository creates a new flashcard repository.
func NewFlashcardRepository(db *database.DB) FlashcardRepository {
	return &flashcardRepository{db: db}
}

// CreateDeck creates a new flashcard deck with cards.
func (r *flashcardRepository) CreateDeck(ctx context.Context, deck *model.FlashcardDeck) error {
	return r.db.DB.WithContext(ctx).Create(deck).Error
}

// GetDeckByID retrieves a deck by ID with cards.
func (r *flashcardRepository) GetDeckByID(ctx context.Context, id uuid.UUID) (*model.FlashcardDeck, error) {
	var deck model.FlashcardDeck
	err := r.db.WithContext(ctx).
		Preload("Course").
		Preload("Material").
		Preload("Flashcards").
		First(&deck, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &deck, nil
}

// GetDecksByUserID retrieves decks for a user.
func (r *flashcardRepository) GetDecksByUserID(ctx context.Context, userID uuid.UUID) ([]model.FlashcardDeck, error) {
	var decks []model.FlashcardDeck
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&decks).Error
	return decks, err
}

// GetDecksByCourseID retrieves decks for a course.
func (r *flashcardRepository) GetDecksByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.FlashcardDeck, error) {
	var decks []model.FlashcardDeck
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Order("created_at DESC").
		Find(&decks).Error
	return decks, err
}

// GetDecksByMaterialID retrieves decks for a material.
func (r *flashcardRepository) GetDecksByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.FlashcardDeck, error) {
	var decks []model.FlashcardDeck
	err := r.db.WithContext(ctx).
		Where("material_id = ?", materialID).
		Order("created_at DESC").
		Find(&decks).Error
	return decks, err
}

// UpdateDeck updates a flashcard deck.
func (r *flashcardRepository) UpdateDeck(ctx context.Context, deck *model.FlashcardDeck) error {
	return r.db.DB.WithContext(ctx).Save(deck).Error
}

// DeleteDeck soft-deletes a deck.
func (r *flashcardRepository) DeleteDeck(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.FlashcardDeck{}, "id = ?", id).Error
}

// CreateCard creates a new flashcard.
func (r *flashcardRepository) CreateCard(ctx context.Context, card *model.Flashcard) error {
	return r.db.DB.WithContext(ctx).Create(card).Error
}

// GetCardsByDeckID retrieves cards for a deck.
func (r *flashcardRepository) GetCardsByDeckID(ctx context.Context, deckID uuid.UUID) ([]model.Flashcard, error) {
	var cards []model.Flashcard
	err := r.db.WithContext(ctx).
		Where("deck_id = ?", deckID).
		Order("order_index").
		Find(&cards).Error
	return cards, err
}

// GetCardByID retrieves a single flashcard by ID.
func (r *flashcardRepository) GetCardByID(ctx context.Context, id uuid.UUID) (*model.Flashcard, error) {
	var card model.Flashcard
	err := r.db.WithContext(ctx).First(&card, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// UpdateCard updates a flashcard.
func (r *flashcardRepository) UpdateCard(ctx context.Context, card *model.Flashcard) error {
	return r.db.DB.WithContext(ctx).Save(card).Error
}

// DeleteCard deletes a flashcard.
func (r *flashcardRepository) DeleteCard(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Delete(&model.Flashcard{}, "id = ?", id).Error
}
