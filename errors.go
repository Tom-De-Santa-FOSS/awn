package awn

import (
	"errors"
	"fmt"
)

// Error categories.
const (
	CategorySession   = "session"
	CategoryDaemon    = "daemon"
	CategoryAuth      = "auth"
	CategoryTimeout   = "timeout"
	CategoryInput     = "input"
	CategoryPipeline  = "pipeline"
	CategoryTransport = "transport"
	CategoryValidation = "validation"
)

// AwnError is a structured error with machine-readable fields.
type AwnError struct {
	Code       string            `json:"code"`
	Category   string            `json:"category"`
	Message    string            `json:"message"`
	Retryable  bool              `json:"retryable"`
	Suggestion string            `json:"suggestion,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
}

func (e *AwnError) Error() string { return e.Message }

// IsRetryable returns true if the error is an AwnError with Retryable set.
// Returns false for non-AwnError errors.
func IsRetryable(err error) bool {
	var ae *AwnError
	if errors.As(err, &ae) {
		return ae.Retryable
	}
	return false
}

// ErrorCode returns the error code if the error is an AwnError, or "" otherwise.
func ErrorCode(err error) string {
	var ae *AwnError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// --- Error constructors ---

// ErrSessionNotFound returns a structured error for missing sessions.
func ErrSessionNotFound(id string) *AwnError {
	return &AwnError{
		Code:       "SESSION_NOT_FOUND",
		Category:   CategorySession,
		Message:    fmt.Sprintf("session %q not found", id),
		Retryable:  false,
		Suggestion: "check session ID with awn list",
		Context:    map[string]string{"session_id": id},
	}
}

// ErrSessionExited returns a structured error for sessions whose process has exited.
func ErrSessionExited(id string) *AwnError {
	return &AwnError{
		Code:       "SESSION_EXITED",
		Category:   CategorySession,
		Message:    fmt.Sprintf("session %q process has exited", id),
		Retryable:  false,
		Suggestion: "create a new session with 'awn create'",
		Context:    map[string]string{"session_id": id},
	}
}

// ErrTimeout returns a structured error for timeout conditions.
func ErrTimeout(operation string, ms int) *AwnError {
	return &AwnError{
		Code:       "TIMEOUT",
		Category:   CategoryTimeout,
		Message:    fmt.Sprintf("%s timed out after %dms", operation, ms),
		Retryable:  true,
		Suggestion: "increase --timeout or check if the expected condition can occur",
	}
}

// ErrValidation returns a structured error for invalid input.
func ErrValidation(msg string) *AwnError {
	return &AwnError{
		Code:       "VALIDATION_ERROR",
		Category:   CategoryValidation,
		Message:    msg,
		Retryable:  false,
		Suggestion: "check command usage",
	}
}

// ErrDaemonNotRunning returns a structured error when the daemon is unreachable.
func ErrDaemonNotRunning() *AwnError {
	return &AwnError{
		Code:       "DAEMON_NOT_RUNNING",
		Category:   CategoryDaemon,
		Message:    "daemon is not running",
		Retryable:  true,
		Suggestion: "start the daemon with 'awn daemon start'",
	}
}

// ErrDaemonAlreadyRunning returns a structured error when trying to start a second daemon.
func ErrDaemonAlreadyRunning() *AwnError {
	return &AwnError{
		Code:       "DAEMON_ALREADY_RUNNING",
		Category:   CategoryDaemon,
		Message:    "daemon is already running",
		Retryable:  false,
		Suggestion: "stop it first with 'awn daemon stop'",
	}
}

// ErrConnectionFailed returns a structured error for connection failures.
func ErrConnectionFailed(detail string) *AwnError {
	return &AwnError{
		Code:       "CONNECTION_FAILED",
		Category:   CategoryTransport,
		Message:    fmt.Sprintf("connection failed: %s", detail),
		Retryable:  true,
		Suggestion: "check that the daemon is running and reachable",
	}
}

// ErrAuthRequired returns a structured error when TCP mode lacks a token.
func ErrAuthRequired() *AwnError {
	return &AwnError{
		Code:       "AUTH_REQUIRED",
		Category:   CategoryAuth,
		Message:    "authentication required for TCP connections",
		Retryable:  false,
		Suggestion: "set AWN_TOKEN environment variable",
	}
}

// ErrAuthFailed returns a structured error for invalid credentials.
func ErrAuthFailed() *AwnError {
	return &AwnError{
		Code:       "AUTH_FAILED",
		Category:   CategoryAuth,
		Message:    "authentication failed",
		Retryable:  false,
		Suggestion: "check that AWN_TOKEN matches on daemon and client",
	}
}

// ErrInvalidKey returns a structured error for unrecognized key names.
func ErrInvalidKey(key string) *AwnError {
	return &AwnError{
		Code:       "INVALID_KEY",
		Category:   CategoryInput,
		Message:    fmt.Sprintf("unknown key: %s", key),
		Retryable:  false,
		Suggestion: "supported keys: Enter, Tab, Backspace, Escape, Space, Delete, Up, Down, Left, Right, Home, End, PageUp, PageDown, F1-F12, Ctrl+A-Z",
		Context:    map[string]string{"key": key},
	}
}

// ErrPipelineStepFailed returns a structured error for a failed pipeline step.
func ErrPipelineStepFailed(step int, detail string) *AwnError {
	return &AwnError{
		Code:       "PIPELINE_STEP_FAILED",
		Category:   CategoryPipeline,
		Message:    fmt.Sprintf("pipeline step %d failed: %s", step, detail),
		Retryable:  false,
		Suggestion: "check the failed step in pipeline output",
		Context:    map[string]string{"step_index": fmt.Sprintf("%d", step)},
	}
}

// ErrInvalidDir returns a structured error for invalid working directory.
func ErrInvalidDir(dir string) *AwnError {
	return &AwnError{
		Code:       "INVALID_INPUT",
		Category:   CategoryValidation,
		Message:    fmt.Sprintf("directory %q does not exist or is not a directory", dir),
		Retryable:  false,
		Suggestion: "provide a valid directory path with --dir",
		Context:    map[string]string{"dir": dir},
	}
}

