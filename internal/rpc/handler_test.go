package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tom/awn"
)

// stubStrategy is a no-op strategy for handler tests.
type stubStrategy struct{}

func (stubStrategy) Detect(screen *awn.Screen) []awn.Element { return nil }

func newTestHandler() *Handler {
	d := awn.NewDriver()
	return NewHandler(d, stubStrategy{})
}

func TestDispatch_UnknownMethod(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("bogus", nil)
	if err == nil || !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("expected method not found error, got: %v", err)
	}
}

func TestDispatch_Create_InvalidParams(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("create", json.RawMessage(`"not json"`))
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Fatalf("expected invalid params error, got: %v", err)
	}
}

func TestDispatch_List(t *testing.T) {
	h := newTestHandler()
	result, err := h.Dispatch("list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*ListResponse)
	if !ok {
		t.Fatalf("expected *ListResponse, got %T", result)
	}
	if resp.Sessions == nil {
		t.Fatalf("expected non-nil sessions slice, got nil")
	}
	if len(resp.Sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(resp.Sessions))
	}
}

func TestDispatch_ScreenshotNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("screenshot", json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}

func TestDispatch_DetectNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("detect", json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}

func TestDispatch_InputNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("input", json.RawMessage(`{"id":"nonexistent","data":"x"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}

func TestDispatch_WaitForTextNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("wait_for_text", json.RawMessage(`{"id":"nonexistent","text":"x"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}

func TestDispatch_WaitForStableNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("wait_for_stable", json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}

func TestDispatch_CloseNotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.Dispatch("close", json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}
