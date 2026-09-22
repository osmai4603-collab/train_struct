package examples

import (
	"strings"

	platformerr "train/internal/platform/errors"
)

// RegisterDTO represents the data for registering a new user
type RegisterDTO struct {
	Email    string
	Password string
	Age      int
}

// ValidateRegisterDTO validates the inputs and collects all the violations
func ValidateRegisterDTO(dto RegisterDTO) error {
	valErr := platformerr.NewValidationError("RegisterDTO")

	if strings.TrimSpace(dto.Email) == "" {
		valErr.AddViolation("email", "email cannot be blank")
	} else if !strings.Contains(dto.Email, "@") {
		valErr.AddViolation("email", "invalid email address format")
	}

	if len(dto.Password) < 8 {
		valErr.AddViolation("password", "password must be at least 8 characters long")
	}

	if dto.Age < 18 {
		valErr.AddViolation("age", "user must be at least 18 years old")
	}

	if valErr.HasViolations() {
		return valErr.AsError("examples.ValidateRegisterDTO")
	}

	return nil
}
