package awn

import "fmt"

// AwnError is a structured error with machine-readable fields.
type AwnError struct {
	Code       string `json:"code"`
	Category   string `json:"category"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (e *AwnError) Error() string { return e.Message }

// ErrSessionNotFound returns a structured error for missing sessions.
func ErrSessionNotFound(id string) *AwnError {
	return &AwnError{
		Code:       "SESSION_NOT_FOUND",
		Category:   "session",
		Message:    fmt.Sprintf("session %q not found", id),
		Retryable:  false,
		Suggestion: "check session ID with awn list",
	}
}

// ErrTimeout returns a structured error for timeout conditions.
func ErrTimeout(operation string, ms int) *AwnError {
	return &AwnError{
		Code:       "TIMEOUT",
		Category:   "timeout",
		Message:    fmt.Sprintf("%s timed out after %dms", operation, ms),
		Retryable:  true,
		Suggestion: "increase --timeout or check if the expected condition can occur",
	}
}

// ErrValidation returns a structured error for invalid input.
func ErrValidation(msg string) *AwnError {
	return &AwnError{
		Code:       "VALIDATION_ERROR",
		Category:   "validation",
		Message:    msg,
		Retryable:  false,
		Suggestion: "check command usage",
	}
}

// ErrSessionNotRunning returns a structured error when a session process has exited.
func ErrSessionNotRunning(id string) *AwnError {
	return &AwnError{
		Code:       "SESSION_NOT_RUNNING",
		Category:   "session",
		Message:    fmt.Sprintf("session %q process is not running", id),
		Retryable:  false,
		Suggestion: "the session process has exited; create a new session",
	}
}

// ErrTerminal returns a structured error for terminal emulation issues.
func ErrTerminal(msg string) *AwnError {
	return &AwnError{
		Code:      "TERMINAL_ERROR",
		Category:  "terminal",
		Message:   msg,
		Retryable: false,
	}
}
