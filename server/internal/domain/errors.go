package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidPrompt      = errors.New("invalid prompt")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrUnsupportedPlan    = errors.New("unsupported plan")
	ErrProviderFailure    = errors.New("provider failure")
	ErrDuplicateOperation = errors.New("duplicate operation")
)
