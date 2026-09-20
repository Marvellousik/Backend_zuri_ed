package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"zuri/services/content/internal/service"
	"zuri/shared/pkg/logger"
)

// FlashcardHandler handles flashcard-related HTTP requests.
type FlashcardHandler struct {
	service   service.ContentService
	validator *validator.Validate
}

// NewFlashcardHandler creates a new flashcard handler.
func NewFlashcardHandler(service service.ContentService) *FlashcardHandler {
	return &FlashcardHandler{
		service:   service,
		validator: validator.New(),
	}
}

// CreateDeck handles POST /api/v1/flashcard-decks
func (h *FlashcardHandler) CreateDeck(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req service.CreateDeckRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deck, err := h.service.CreateFlashcardDeck(c.Request().Context(), userID, &req)
	if err != nil {
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to create deck", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create deck")
	}

	return c.JSON(http.StatusCreated, Response{Data: deck})
}

// GetDeck handles GET /api/v1/flashcard-decks/:id
func (h *FlashcardHandler) GetDeck(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	deckID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid deck ID")
	}

	deck, err := h.service.GetFlashcardDeck(c.Request().Context(), userID, deckID)
	if err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "deck not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to get deck", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get deck")
	}

	return c.JSON(http.StatusOK, Response{Data: deck})
}

// GetUserDecks handles GET /api/v1/flashcard-decks
func (h *FlashcardHandler) GetUserDecks(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	decks, err := h.service.GetUserFlashcardDecks(c.Request().Context(), userID)
	if err != nil {
		logger.Error("failed to get decks", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get decks")
	}

	return c.JSON(http.StatusOK, Response{Data: decks})
}

// UpdateDeck handles PUT /api/v1/flashcard-decks/:id
func (h *FlashcardHandler) UpdateDeck(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	deckID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid deck ID")
	}

	var req service.UpdateDeckRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deck, err := h.service.UpdateFlashcardDeck(c.Request().Context(), userID, deckID, &req)
	if err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "deck not found")
		}
		if err == service.ErrCourseNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "course not found")
		}
		if err == service.ErrMaterialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "material not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update deck", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update deck")
	}

	return c.JSON(http.StatusOK, Response{Data: deck})
}

// DeleteDeck handles DELETE /api/v1/flashcard-decks/:id
func (h *FlashcardHandler) DeleteDeck(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	deckID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid deck ID")
	}

	if err := h.service.DeleteFlashcardDeck(c.Request().Context(), userID, deckID); err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "deck not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete deck", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete deck")
	}

	return c.NoContent(http.StatusNoContent)
}

// AddCard handles POST /api/v1/flashcard-decks/:id/cards
func (h *FlashcardHandler) AddCard(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	deckID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid deck ID")
	}

	var req service.AddFlashcardRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.validator.Struct(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	card, err := h.service.AddFlashcard(c.Request().Context(), userID, deckID, &req)
	if err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "deck not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to add card", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add card")
	}

	return c.JSON(http.StatusCreated, Response{Data: card})
}

// UpdateCard handles PUT /api/v1/flashcards/:id
func (h *FlashcardHandler) UpdateCard(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	cardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid card ID")
	}

	var req service.UpdateFlashcardRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	card, err := h.service.UpdateFlashcard(c.Request().Context(), userID, cardID, &req)
	if err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "card not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to update card", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update card")
	}

	return c.JSON(http.StatusOK, Response{Data: card})
}

// DeleteCard handles DELETE /api/v1/flashcards/:id
func (h *FlashcardHandler) DeleteCard(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	cardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid card ID")
	}

	if err := h.service.DeleteFlashcard(c.Request().Context(), userID, cardID); err != nil {
		if err == service.ErrFlashcardNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "card not found")
		}
		if err == service.ErrUnauthorized {
			return echo.NewHTTPError(http.StatusForbidden, "access denied")
		}
		logger.Error("failed to delete card", zap.Error(err))
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete card")
	}

	return c.NoContent(http.StatusNoContent)
}
