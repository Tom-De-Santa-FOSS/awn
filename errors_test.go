package awn

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestAwnError_Error(t *testing.T) {
	e := &AwnError{
		Code:    "SESSION_NOT_FOUND",
		Message: "session \"abc\" not found",
	}
	if e.Error() != `session "abc" not found` {
		t.Fatalf("Error() = %q", e.Error())
	}
}

func TestAwnError_JSON(t *testing.T) {
	e := &AwnError{
		Code:       "SESSION_NOT_FOUND",
		Category:   "session",
		Message:    "session not found",
		Retryable:  false,
		Suggestion: "check session ID with awn list",
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	json.Unmarshal(data, &got) //nolint:errcheck
	if got["code"] != "SESSION_NOT_FOUND" {
		t.Fatalf("code = %v", got["code"])
	}
	if got["category"] != "session" {
		t.Fatalf("category = %v", got["category"])
	}
	if got["suggestion"] != "check session ID with awn list" {
		t.Fatalf("suggestion = %v", got["suggestion"])
	}
}

func TestAwnError_JSONWithContext(t *testing.T) {
	e := ErrSessionNotFound("abc123")
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	json.Unmarshal(data, &got) //nolint:errcheck
	ctx, ok := got["context"].(map[string]any)
	if !ok {
		t.Fatal("context field missing or not an object")
	}
	if ctx["session_id"] != "abc123" {
		t.Fatalf("context.session_id = %v", ctx["session_id"])
	}
}

func TestAwnError_JSONOmitsEmptyContext(t *testing.T) {
	e := &AwnError{Code: "TEST", Message: "test"}
	data, _ := json.Marshal(e)
	var got map[string]any
	json.Unmarshal(data, &got) //nolint:errcheck
	if _, exists := got["context"]; exists {
		t.Fatal("context should be omitted when nil")
	}
}

func TestErrSessionNotFound(t *testing.T) {
	e := ErrSessionNotFound("abc123")
	if e.Code != "SESSION_NOT_FOUND" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Category != CategorySession {
		t.Fatalf("Category = %q", e.Category)
	}
	if e.Retryable {
		t.Fatal("should not be retryable")
	}
	if e.Context["session_id"] != "abc123" {
		t.Fatalf("Context[session_id] = %q", e.Context["session_id"])
	}
}

func TestErrSessionExited(t *testing.T) {
	e := ErrSessionExited("xyz")
	if e.Code != "SESSION_EXITED" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Retryable {
		t.Fatal("should not be retryable")
	}
}

func TestErrTimeout(t *testing.T) {
	e := ErrTimeout("wait_for_text", 5000)
	if e.Code != "TIMEOUT" {
		t.Fatalf("Code = %q", e.Code)
	}
	if !e.Retryable {
		t.Fatal("should be retryable")
	}
}

func TestErrValidation(t *testing.T) {
	e := ErrValidation("missing field: text")
	if e.Code != "VALIDATION_ERROR" {
		t.Fatalf("Code = %q", e.Code)
	}
}

func TestErrDaemonNotRunning(t *testing.T) {
	e := ErrDaemonNotRunning()
	if e.Code != "DAEMON_NOT_RUNNING" {
		t.Fatalf("Code = %q", e.Code)
	}
	if !e.Retryable {
		t.Fatal("should be retryable")
	}
}

func TestErrDaemonAlreadyRunning(t *testing.T) {
	e := ErrDaemonAlreadyRunning()
	if e.Code != "DAEMON_ALREADY_RUNNING" {
		t.Fatalf("Code = %q", e.Code)
	}
}

func TestErrAuthRequired(t *testing.T) {
	e := ErrAuthRequired()
	if e.Code != "AUTH_REQUIRED" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Category != CategoryAuth {
		t.Fatalf("Category = %q", e.Category)
	}
}

func TestErrAuthFailed(t *testing.T) {
	e := ErrAuthFailed()
	if e.Code != "AUTH_FAILED" {
		t.Fatalf("Code = %q", e.Code)
	}
}

func TestErrInvalidKey(t *testing.T) {
	e := ErrInvalidKey("FooBar")
	if e.Code != "INVALID_KEY" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Context["key"] != "FooBar" {
		t.Fatalf("Context[key] = %q", e.Context["key"])
	}
}

func TestErrPipelineStepFailed(t *testing.T) {
	e := ErrPipelineStepFailed(3, "timeout")
	if e.Code != "PIPELINE_STEP_FAILED" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Context["step_index"] != "3" {
		t.Fatalf("Context[step_index] = %q", e.Context["step_index"])
	}
}

func TestErrConnectionFailed(t *testing.T) {
	e := ErrConnectionFailed("dial timeout")
	if e.Code != "CONNECTION_FAILED" {
		t.Fatalf("Code = %q", e.Code)
	}
	if !e.Retryable {
		t.Fatal("should be retryable")
	}
}

func TestErrInvalidDir(t *testing.T) {
	e := ErrInvalidDir("/no/such/path")
	if e.Code != "INVALID_INPUT" {
		t.Fatalf("Code = %q", e.Code)
	}
	if e.Context["dir"] != "/no/such/path" {
		t.Fatalf("Context[dir] = %q", e.Context["dir"])
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout is retryable", ErrTimeout("test", 1000), true},
		{"session not found is not retryable", ErrSessionNotFound("x"), false},
		{"daemon not running is retryable", ErrDaemonNotRunning(), true},
		{"connection failed is retryable", ErrConnectionFailed("x"), true},
		{"validation is not retryable", ErrValidation("x"), false},
		{"plain error is not retryable", fmt.Errorf("boom"), false},
		{"nil is not retryable", nil, false},
		{"wrapped AwnError", fmt.Errorf("wrap: %w", ErrTimeout("x", 1)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorCode(t *testing.T) {
	if got := ErrorCode(ErrTimeout("x", 1)); got != "TIMEOUT" {
		t.Errorf("ErrorCode = %q, want TIMEOUT", got)
	}
	if got := ErrorCode(fmt.Errorf("plain")); got != "" {
		t.Errorf("ErrorCode = %q, want empty", got)
	}
}

func TestErrorsAs(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", ErrSessionNotFound("test"))
	var ae *AwnError
	if !errors.As(err, &ae) {
		t.Fatal("errors.As should unwrap AwnError")
	}
	if ae.Code != "SESSION_NOT_FOUND" {
		t.Fatalf("Code = %q", ae.Code)
	}
}

func TestCategoryConstants(t *testing.T) {
	// Verify categories are used consistently in constructors.
	cases := []struct {
		name     string
		err      *AwnError
		category string
	}{
		{"session", ErrSessionNotFound("x"), CategorySession},
		{"timeout", ErrTimeout("x", 1), CategoryTimeout},
		{"validation", ErrValidation("x"), CategoryValidation},
		{"daemon", ErrDaemonNotRunning(), CategoryDaemon},
		{"auth", ErrAuthRequired(), CategoryAuth},
		{"transport", ErrConnectionFailed("x"), CategoryTransport},
		{"input", ErrInvalidKey("x"), CategoryInput},
		{"pipeline", ErrPipelineStepFailed(0, "x"), CategoryPipeline},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Category != tt.category {
				t.Errorf("Category = %q, want %q", tt.err.Category, tt.category)
			}
		})
	}
}
