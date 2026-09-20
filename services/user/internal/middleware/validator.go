// Package middleware contains HTTP middleware for the User Service.
package middleware

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomValidator is a custom Echo validator using go-playground/validator.
type CustomValidator struct {
	validator *validator.Validate
}

// NewValidator creates a new custom validator.
func NewValidator() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

// Validate validates the input struct.
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// Register registers the custom validator with Echo.
func Register(e *echo.Echo) {
	e.Validator = NewValidator()
}
