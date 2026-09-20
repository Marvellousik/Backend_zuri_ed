package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuri/services/content/internal/model"
	"zuri/services/content/internal/service"
)

// ==================== Mocks ====================

type mockCourseRepo struct {
	courses map[uuid.UUID]*model.Course
}

func newMockCourseRepo() *mockCourseRepo {
	return &mockCourseRepo{courses: make(map[uuid.UUID]*model.Course)}
}

func (m *mockCourseRepo) Create(ctx context.Context, course *model.Course) error {
	if course.ID == uuid.Nil {
		course.ID = uuid.New()
	}
	m.courses[course.ID] = course
	return nil
}

func (m *mockCourseRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Course, error) {
	c, ok := m.courses[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return c, nil
}

func (m *mockCourseRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Course, error) {
	var list []model.Course
	for _, c := range m.courses {
		if c.UserID == userID {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (m *mockCourseRepo) Update(ctx context.Context, course *model.Course) error {
	m.courses[course.ID] = course
	return nil
}

func (m *mockCourseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.courses, id)
	return nil
}

type mockMaterialRepo struct {
	materials map[uuid.UUID]*model.Material
}

func newMockMaterialRepo() *mockMaterialRepo {
	return &mockMaterialRepo{materials: make(map[uuid.UUID]*model.Material)}
}

func (m *mockMaterialRepo) Create(ctx context.Context, mat *model.Material) error {
	if mat.ID == uuid.Nil {
		mat.ID = uuid.New()
	}
	m.materials[mat.ID] = mat
	return nil
}

func (m *mockMaterialRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Material, error) {
	mat, ok := m.materials[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return mat, nil
}

func (m *mockMaterialRepo) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Material, error) {
	var list []model.Material
	for _, mat := range m.materials {
		if mat.UserID == userID {
			list = append(list, *mat)
		}
	}
	return list, nil
}

func (m *mockMaterialRepo) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Material, error) {
	var list []model.Material
	for _, mat := range m.materials {
		if mat.CourseID != nil && *mat.CourseID == courseID {
			list = append(list, *mat)
		}
	}
	return list, nil
}

func (m *mockMaterialRepo) Update(ctx context.Context, mat *model.Material) error {
	m.materials[mat.ID] = mat
	return nil
}

func (m *mockMaterialRepo) UpdateProcessingStatus(ctx context.Context, id uuid.UUID, status model.ProcessingStatus, summary string) error {
	if mat, ok := m.materials[id]; ok {
		mat.ProcessingStatus = status
		mat.Summary = summary
	}
	return nil
}

func (m *mockMaterialRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.materials, id)
	return nil
}

type mockQuizRepo struct {
	quizzes               map[uuid.UUID]*model.Quiz
	questions             map[uuid.UUID]*model.QuizQuestion
	lastQueriedQuestionID uuid.UUID
}

func newMockQuizRepo() *mockQuizRepo {
	return &mockQuizRepo{
		quizzes:   make(map[uuid.UUID]*model.Quiz),
		questions: make(map[uuid.UUID]*model.QuizQuestion),
	}
}

func (m *mockQuizRepo) Create(ctx context.Context, quiz *model.Quiz) error {
	if quiz.ID == uuid.Nil {
		quiz.ID = uuid.New()
	}
	m.quizzes[quiz.ID] = quiz
	return nil
}

func (m *mockQuizRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Quiz, error) {
	q, ok := m.quizzes[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return q, nil
}

func (m *mockQuizRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Quiz, error) {
	var list []model.Quiz
	for _, q := range m.quizzes {
		if q.UserID == userID {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockQuizRepo) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.Quiz, error) {
	var list []model.Quiz
	for _, q := range m.quizzes {
		if q.CourseID != nil && *q.CourseID == courseID {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockQuizRepo) GetByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.Quiz, error) {
	var list []model.Quiz
	for _, q := range m.quizzes {
		if q.MaterialID != nil && *q.MaterialID == materialID {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockQuizRepo) Update(ctx context.Context, quiz *model.Quiz) error {
	m.quizzes[quiz.ID] = quiz
	return nil
}

func (m *mockQuizRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.quizzes, id)
	return nil
}

func (m *mockQuizRepo) CreateQuestion(ctx context.Context, question *model.QuizQuestion) error {
	if question.ID == uuid.Nil {
		question.ID = uuid.New()
	}
	m.questions[question.ID] = question
	return nil
}

func (m *mockQuizRepo) GetQuestionsByQuizID(ctx context.Context, quizID uuid.UUID) ([]model.QuizQuestion, error) {
	m.lastQueriedQuestionID = quizID
	var list []model.QuizQuestion
	for _, q := range m.questions {
		if q.QuizID == quizID {
			list = append(list, *q)
		}
	}
	return list, nil
}

func (m *mockQuizRepo) GetQuestionByID(ctx context.Context, id uuid.UUID) (*model.QuizQuestion, error) {
	m.lastQueriedQuestionID = id
	q, ok := m.questions[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return q, nil
}

func (m *mockQuizRepo) UpdateQuestion(ctx context.Context, question *model.QuizQuestion) error {
	m.questions[question.ID] = question
	return nil
}

func (m *mockQuizRepo) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	delete(m.questions, id)
	return nil
}

type mockFlashcardRepo struct {
	decks map[uuid.UUID]*model.FlashcardDeck
	cards map[uuid.UUID]*model.Flashcard
}

func newMockFlashcardRepo() *mockFlashcardRepo {
	return &mockFlashcardRepo{
		decks: make(map[uuid.UUID]*model.FlashcardDeck),
		cards: make(map[uuid.UUID]*model.Flashcard),
	}
}

func (m *mockFlashcardRepo) CreateDeck(ctx context.Context, deck *model.FlashcardDeck) error {
	if deck.ID == uuid.Nil {
		deck.ID = uuid.New()
	}
	m.decks[deck.ID] = deck
	return nil
}

func (m *mockFlashcardRepo) GetDeckByID(ctx context.Context, id uuid.UUID) (*model.FlashcardDeck, error) {
	d, ok := m.decks[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return d, nil
}

func (m *mockFlashcardRepo) GetDecksByUserID(ctx context.Context, userID uuid.UUID) ([]model.FlashcardDeck, error) {
	var list []model.FlashcardDeck
	for _, d := range m.decks {
		if d.UserID == userID {
			list = append(list, *d)
		}
	}
	return list, nil
}

func (m *mockFlashcardRepo) GetDecksByCourseID(ctx context.Context, courseID uuid.UUID) ([]model.FlashcardDeck, error) {
	var list []model.FlashcardDeck
	for _, d := range m.decks {
		if d.CourseID != nil && *d.CourseID == courseID {
			list = append(list, *d)
		}
	}
	return list, nil
}

func (m *mockFlashcardRepo) GetDecksByMaterialID(ctx context.Context, materialID uuid.UUID) ([]model.FlashcardDeck, error) {
	var list []model.FlashcardDeck
	for _, d := range m.decks {
		if d.MaterialID != nil && *d.MaterialID == materialID {
			list = append(list, *d)
		}
	}
	return list, nil
}

func (m *mockFlashcardRepo) UpdateDeck(ctx context.Context, deck *model.FlashcardDeck) error {
	m.decks[deck.ID] = deck
	return nil
}

func (m *mockFlashcardRepo) DeleteDeck(ctx context.Context, id uuid.UUID) error {
	delete(m.decks, id)
	return nil
}

func (m *mockFlashcardRepo) CreateCard(ctx context.Context, card *model.Flashcard) error {
	if card.ID == uuid.Nil {
		card.ID = uuid.New()
	}
	m.cards[card.ID] = card
	return nil
}

func (m *mockFlashcardRepo) GetCardsByDeckID(ctx context.Context, deckID uuid.UUID) ([]model.Flashcard, error) {
	var list []model.Flashcard
	for _, c := range m.cards {
		if c.DeckID == deckID {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (m *mockFlashcardRepo) GetCardByID(ctx context.Context, id uuid.UUID) (*model.Flashcard, error) {
	c, ok := m.cards[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	return c, nil
}

func (m *mockFlashcardRepo) UpdateCard(ctx context.Context, card *model.Flashcard) error {
	m.cards[card.ID] = card
	return nil
}

func (m *mockFlashcardRepo) DeleteCard(ctx context.Context, id uuid.UUID) error {
	delete(m.cards, id)
	return nil
}

// ==================== Test Helper ====================

type testContext struct {
	svc           service.ContentService
	courseRepo    *mockCourseRepo
	materialRepo  *mockMaterialRepo
	quizRepo      *mockQuizRepo
	flashcardRepo *mockFlashcardRepo
}

func setupTestContext() *testContext {
	cRepo := newMockCourseRepo()
	mRepo := newMockMaterialRepo()
	qRepo := newMockQuizRepo()
	fRepo := newMockFlashcardRepo()

	svc := service.NewContentService(cRepo, mRepo, qRepo, fRepo, nil, "test-bucket", "")
	return &testContext{
		svc:           svc,
		courseRepo:    cRepo,
		materialRepo:  mRepo,
		quizRepo:      qRepo,
		flashcardRepo: fRepo,
	}
}

// ==================== Tests ====================

func TestCourseBindingIDOR_CreateMaterial(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	// User A creates a course
	courseA := &model.Course{
		ID:        uuid.New(),
		UserID:    userA,
		Name:      "Algorithms 101",
		CreatedAt: time.Now(),
	}
	require.NoError(t, tc.courseRepo.Create(ctx, courseA))

	t.Run("Owner can bind material to their own course", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			CourseID: &courseA.ID,
			Title:    "Lecture 1 Notes",
		}
		mat, err := tc.svc.CreateMaterial(ctx, userA, req)
		require.NoError(t, err)
		assert.Equal(t, courseA.ID, *mat.CourseID)
		assert.Equal(t, userA, mat.UserID)
	})

	t.Run("Non-owner is rejected when attempting to bind to User A course (IDOR)", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			CourseID: &courseA.ID,
			Title:    "Malicious Attachment",
		}
		mat, err := tc.svc.CreateMaterial(ctx, userB, req)
		assert.Nil(t, mat)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("Non-existent course returns ErrCourseNotFound", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := &service.CreateMaterialRequest{
			CourseID: &nonExistentID,
			Title:    "Ghost Course Material",
		}
		mat, err := tc.svc.CreateMaterial(ctx, userA, req)
		assert.Nil(t, mat)
		assert.ErrorIs(t, err, service.ErrCourseNotFound)
	})
}

func TestCourseBindingIDOR_UpdateMaterial(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	courseA := &model.Course{
		ID:     uuid.New(),
		UserID: userA,
		Name:   "Biology",
	}
	require.NoError(t, tc.courseRepo.Create(ctx, courseA))

	// User B creates material without course
	matB := &model.Material{
		ID:     uuid.New(),
		UserID: userB,
		Title:  "User B Note",
	}
	require.NoError(t, tc.materialRepo.Create(ctx, matB))

	t.Run("User B cannot update material to bind to User A course", func(t *testing.T) {
		req := &service.UpdateMaterialRequest{
			CourseID: &courseA.ID,
		}
		mat, err := tc.svc.UpdateMaterial(ctx, userB, matB.ID, req)
		assert.Nil(t, mat)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})
}

func TestCourseBindingIDOR_CreateQuiz(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	courseA := &model.Course{
		ID:     uuid.New(),
		UserID: userA,
		Name:   "Calculus",
	}
	require.NoError(t, tc.courseRepo.Create(ctx, courseA))

	matA := &model.Material{
		ID:     uuid.New(),
		UserID: userA,
		Title:  "Derivatives PDF",
	}
	require.NoError(t, tc.materialRepo.Create(ctx, matA))

	t.Run("User A can create quiz bound to their course and material", func(t *testing.T) {
		req := &service.CreateQuizRequest{
			CourseID:   &courseA.ID,
			MaterialID: &matA.ID,
			Title:      "Calculus Quiz 1",
		}
		quiz, err := tc.svc.CreateQuiz(ctx, userA, req)
		require.NoError(t, err)
		assert.Equal(t, courseA.ID, *quiz.CourseID)
	})

	t.Run("User B cannot bind quiz to User A course", func(t *testing.T) {
		req := &service.CreateQuizRequest{
			CourseID: &courseA.ID,
			Title:    "Attacker Quiz",
		}
		quiz, err := tc.svc.CreateQuiz(ctx, userB, req)
		assert.Nil(t, quiz)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("User B cannot bind quiz to User A material", func(t *testing.T) {
		req := &service.CreateQuizRequest{
			MaterialID: &matA.ID,
			Title:      "Attacker Quiz",
		}
		quiz, err := tc.svc.CreateQuiz(ctx, userB, req)
		assert.Nil(t, quiz)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})
}

func TestCourseBindingIDOR_UpdateQuiz(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	courseA := &model.Course{
		ID:     uuid.New(),
		UserID: userA,
		Name:   "Chemistry",
	}
	require.NoError(t, tc.courseRepo.Create(ctx, courseA))

	quizB := &model.Quiz{
		ID:     uuid.New(),
		UserID: userB,
		Title:  "User B Quiz",
	}
	require.NoError(t, tc.quizRepo.Create(ctx, quizB))

	t.Run("User B cannot update their quiz to bind to User A course", func(t *testing.T) {
		req := &service.UpdateQuizRequest{
			CourseID: &courseA.ID,
		}
		updated, err := tc.svc.UpdateQuiz(ctx, userB, quizB.ID, req)
		assert.Nil(t, updated)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})
}

func TestCourseBindingIDOR_FlashcardDeck(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	courseA := &model.Course{
		ID:     uuid.New(),
		UserID: userA,
		Name:   "Physics",
	}
	require.NoError(t, tc.courseRepo.Create(ctx, courseA))

	t.Run("User B cannot create deck bound to User A course", func(t *testing.T) {
		req := &service.CreateDeckRequest{
			CourseID: &courseA.ID,
			Title:    "Physics Cards",
		}
		deck, err := tc.svc.CreateFlashcardDeck(ctx, userB, req)
		assert.Nil(t, deck)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})

	deckB := &model.FlashcardDeck{
		ID:     uuid.New(),
		UserID: userB,
		Title:  "User B Deck",
	}
	require.NoError(t, tc.flashcardRepo.CreateDeck(ctx, deckB))

	t.Run("User B cannot update deck to bind to User A course", func(t *testing.T) {
		req := &service.UpdateDeckRequest{
			CourseID: &courseA.ID,
		}
		deck, err := tc.svc.UpdateFlashcardDeck(ctx, userB, deckB.ID, req)
		assert.Nil(t, deck)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})
}

func TestQuizQuestion_UUIDFix(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	quizA := &model.Quiz{
		ID:     uuid.New(),
		UserID: userA,
		Title:  "History Quiz",
	}
	require.NoError(t, tc.quizRepo.Create(ctx, quizA))

	targetQuestionID := uuid.New()
	questionA := &model.QuizQuestion{
		ID:           targetQuestionID,
		QuizID:       quizA.ID,
		QuestionText: "What year was the Moon landing?",
		QuestionType: model.QuestionTypeShortAnswer,
		Points:       5,
	}
	require.NoError(t, tc.quizRepo.CreateQuestion(ctx, questionA))

	t.Run("UpdateQuizQuestion correctly queries by question ID (not uuid.Nil)", func(t *testing.T) {
		req := &service.UpdateQuestionRequest{
			QuestionText: "What year was Apollo 11?",
			Points:       10,
		}
		updated, err := tc.svc.UpdateQuizQuestion(ctx, userA, targetQuestionID, req)
		require.NoError(t, err)
		assert.Equal(t, targetQuestionID, tc.quizRepo.lastQueriedQuestionID)
		assert.NotEqual(t, uuid.Nil, tc.quizRepo.lastQueriedQuestionID)
		assert.Equal(t, "What year was Apollo 11?", updated.QuestionText)
		assert.Equal(t, 10, updated.Points)
	})

	t.Run("UpdateQuizQuestion fails with ErrUnauthorized when executed by another user", func(t *testing.T) {
		req := &service.UpdateQuestionRequest{
			QuestionText: "Hacked Question",
		}
		updated, err := tc.svc.UpdateQuizQuestion(ctx, userB, targetQuestionID, req)
		assert.Nil(t, updated)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("UpdateQuizQuestion returns ErrQuestionNotFound for non-existent ID", func(t *testing.T) {
		req := &service.UpdateQuestionRequest{
			QuestionText: "Ghost Question",
		}
		updated, err := tc.svc.UpdateQuizQuestion(ctx, userA, uuid.New(), req)
		assert.Nil(t, updated)
		assert.ErrorIs(t, err, service.ErrQuestionNotFound)
	})

	t.Run("DeleteQuizQuestion fails with ErrUnauthorized when executed by another user", func(t *testing.T) {
		err := tc.svc.DeleteQuizQuestion(ctx, userB, targetQuestionID)
		assert.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("DeleteQuizQuestion correctly queries by question ID (not uuid.Nil) and deletes", func(t *testing.T) {
		err := tc.svc.DeleteQuizQuestion(ctx, userA, targetQuestionID)
		require.NoError(t, err)
		assert.Equal(t, targetQuestionID, tc.quizRepo.lastQueriedQuestionID)
		assert.NotEqual(t, uuid.Nil, tc.quizRepo.lastQueriedQuestionID)

		// Verify deletion
		_, err = tc.quizRepo.GetQuestionByID(ctx, targetQuestionID)
		assert.Error(t, err)
	})

	t.Run("DeleteQuizQuestion returns ErrQuestionNotFound for non-existent ID", func(t *testing.T) {
		err := tc.svc.DeleteQuizQuestion(ctx, userA, uuid.New())
		assert.ErrorIs(t, err, service.ErrQuestionNotFound)
	})
}

func TestCourseIntelligence_InstitutionalAndCohortBinding(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()
	user := uuid.New()

	req := &service.CreateCourseRequest{
		InstitutionID:    "veritas_uni",
		Code:             "CSC 301",
		CourseOfferingID: "offering_csc301_2025_first",
		Name:             "Data Structures and Algorithms",
		Description:      "Core 300-level computer science course",
		Color:            "#1E40AF",
		Semester:         "2025/2026_FIRST",
		Year:             2025,
		CreditUnits:      3,
		Syllabus:         `[{"topic": "Recursion", "subtopics": ["Base Cases", "Memoization"]}]`,
	}

	course, err := tc.svc.CreateCourse(ctx, user, req)
	require.NoError(t, err)
	require.NotNil(t, course)

	assert.Equal(t, "veritas_uni", course.InstitutionID)
	assert.Equal(t, "CSC 301", course.Code)
	assert.Equal(t, "offering_csc301_2025_first", course.CourseOfferingID)
	assert.Equal(t, 3, course.CreditUnits)
	assert.Contains(t, course.Syllabus, "Recursion")

	// Update syllabus and course offering
	updateReq := &service.UpdateCourseRequest{
		CourseOfferingID: "offering_csc301_2025_second",
		CreditUnits:      4,
		Syllabus:         `[{"topic": "Graphs", "subtopics": ["DFS", "BFS"]}]`,
	}

	updated, err := tc.svc.UpdateCourse(ctx, user, course.ID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "offering_csc301_2025_second", updated.CourseOfferingID)
	assert.Equal(t, 4, updated.CreditUnits)
	assert.Contains(t, updated.Syllabus, "Graphs")
}

func TestDurableContentProvenance_Sha256AndOfferingBinding(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()
	user := uuid.New()

	const initialSHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	const offeringID = "offering_bio101_fall2026"

	t.Run("CreateMaterial with SHA-256 provenance and course offering binding", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:            "Cell Structure Lecture Notes",
			FileURL:          "https://storage.zuri.local/materials/bio101_cell.pdf",
			FileSize:         1048576,
			MimeType:         "application/pdf",
			Sha256Checksum:   initialSHA,
			CourseOfferingID: offeringID,
			DurationSeconds:  3600,
			Version:          1,
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.Equal(t, initialSHA, mat.Sha256Checksum)
		assert.Equal(t, offeringID, mat.CourseOfferingID)
		assert.Equal(t, 1, mat.Version)
		assert.Equal(t, 3600, mat.DurationSeconds)
		assert.Equal(t, "none", mat.TranscriptionStatus)
	})

	t.Run("UpdateMaterial preserves provenance and updates course offering binding", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:            "Original Document",
			Sha256Checksum:   initialSHA,
			CourseOfferingID: offeringID,
		}
		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)

		const updatedSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		const newOfferingID = "offering_bio101_spring2027"

		updateReq := &service.UpdateMaterialRequest{
			Sha256Checksum:   updatedSHA,
			CourseOfferingID: newOfferingID,
			Version:          2,
			DurationSeconds:  5400,
		}

		updated, err := tc.svc.UpdateMaterial(ctx, user, mat.ID, updateReq)
		require.NoError(t, err)
		require.NotNil(t, updated)

		assert.Equal(t, updatedSHA, updated.Sha256Checksum)
		assert.Equal(t, newOfferingID, updated.CourseOfferingID)
		assert.Equal(t, 2, updated.Version)
		assert.Equal(t, 5400, updated.DurationSeconds)
	})
}

func TestAudioTracking_GenerationAndStatus(t *testing.T) {
	tc := setupTestContext()
	ctx := context.Background()
	user := uuid.New()

	t.Run("AudioURL triggers unique tracking_id generation and sets pending transcription_status", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:    "Recorded Lecture Audio",
			AudioURL: "https://storage.zuri.local/audio/lecture1.mp3",
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.NotEmpty(t, mat.TrackingID)
		assert.Contains(t, mat.TrackingID, "trk_")
		assert.Equal(t, "pending", mat.TranscriptionStatus)
		assert.Equal(t, req.AudioURL, mat.AudioURL)
	})

	t.Run("Audio MIME type triggers tracking_id and pending status without explicit AudioURL", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:    "Podcast Audio File",
			FileURL:  "https://storage.zuri.local/files/episode.bin",
			MimeType: "audio/mpeg",
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.NotEmpty(t, mat.TrackingID)
		assert.Contains(t, mat.TrackingID, "trk_")
		assert.Equal(t, "pending", mat.TranscriptionStatus)
	})

	t.Run("Audio file extension triggers tracking_id and pending status", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:   "Voice Memo",
			FileURL: "https://storage.zuri.local/recordings/memo.wav",
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.NotEmpty(t, mat.TrackingID)
		assert.Equal(t, "pending", mat.TranscriptionStatus)
	})

	t.Run("Non-audio file does not generate tracking_id and defaults to none transcription_status", func(t *testing.T) {
		req := &service.CreateMaterialRequest{
			Title:    "Syllabus PDF",
			FileURL:  "https://storage.zuri.local/files/syllabus.pdf",
			MimeType: "application/pdf",
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.Empty(t, mat.TrackingID)
		assert.Equal(t, "none", mat.TranscriptionStatus)
	})

	t.Run("Explicit tracking_id is preserved when provided with audio", func(t *testing.T) {
		const customTrackingID = "custom-audio-tracking-id-12345"
		req := &service.CreateMaterialRequest{
			Title:      "Lab Session Audio",
			AudioURL:   "https://storage.zuri.local/audio/lab.ogg",
			TrackingID: customTrackingID,
		}

		mat, err := tc.svc.CreateMaterial(ctx, user, req)
		require.NoError(t, err)
		require.NotNil(t, mat)

		assert.Equal(t, customTrackingID, mat.TrackingID)
		assert.Equal(t, "pending", mat.TranscriptionStatus)
	})

	t.Run("UpdateMaterial with AudioURL generates tracking_id if empty and sets status to pending", func(t *testing.T) {
		initialReq := &service.CreateMaterialRequest{
			Title:    "Text Document",
			FileURL:  "https://storage.zuri.local/docs/doc.txt",
			MimeType: "text/plain",
		}
		mat, err := tc.svc.CreateMaterial(ctx, user, initialReq)
		require.NoError(t, err)
		assert.Empty(t, mat.TrackingID)
		assert.Equal(t, "none", mat.TranscriptionStatus)

		updateReq := &service.UpdateMaterialRequest{
			AudioURL: "https://storage.zuri.local/audio/doc_narrated.mp3",
		}
		updated, err := tc.svc.UpdateMaterial(ctx, user, mat.ID, updateReq)
		require.NoError(t, err)
		require.NotNil(t, updated)

		assert.NotEmpty(t, updated.TrackingID)
		assert.Contains(t, updated.TrackingID, "trk_")
		assert.Equal(t, "pending", updated.TranscriptionStatus)
	})

	t.Run("UpdateMaterial allows completing transcription with text", func(t *testing.T) {
		createReq := &service.CreateMaterialRequest{
			Title:    "Lecture Voice Note",
			AudioURL: "https://storage.zuri.local/audio/note.m4a",
		}
		mat, err := tc.svc.CreateMaterial(ctx, user, createReq)
		require.NoError(t, err)
		existingTrackingID := mat.TrackingID

		const transcribedText = "Welcome to the introduction to neural networks and gradient descent."
		updateReq := &service.UpdateMaterialRequest{
			TranscriptionStatus: "completed",
			TranscriptionText:   transcribedText,
		}
		updated, err := tc.svc.UpdateMaterial(ctx, user, mat.ID, updateReq)
		require.NoError(t, err)
		require.NotNil(t, updated)

		assert.Equal(t, existingTrackingID, updated.TrackingID)
		assert.Equal(t, "completed", updated.TranscriptionStatus)
		assert.Equal(t, transcribedText, updated.TranscriptionText)
	})
}
