package awn

import (
	"encoding/json"
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
	json.Unmarshal(data, &got)
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

func TestErrSessionNotFound(t *testing.T) {
	e := ErrSessionNotFound("abc123")
	if e.Code != "SESSION_NOT_FOUND" {
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
