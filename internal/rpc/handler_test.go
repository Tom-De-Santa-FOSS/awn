package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tom/awn/internal/session"
)

func TestDispatch_UnknownMethod(t *testing.T) {
	mgr := session.NewManager()
	h := NewHandler(mgr)
	_, err := h.Dispatch("bogus", nil)
	if err == nil || !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("expected method not found error, got: %v", err)
	}
}

func TestDispatch_InvalidParams(t *testing.T) {
	mgr := session.NewManager()
	h := NewHandler(mgr)
	// Pass a raw string (not a JSON object) as params for the create method.
	_, err := h.Dispatch("create", json.RawMessage(`"not json"`))
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Fatalf("expected invalid params error, got: %v", err)
	}
}

func TestDispatch_List(t *testing.T) {
	mgr := session.NewManager()
	h := NewHandler(mgr)
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
	mgr := session.NewManager()
	h := NewHandler(mgr)
	_, err := h.Dispatch("screenshot", json.RawMessage(`{"id":"nonexistent"}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent session, got nil")
	}
}
