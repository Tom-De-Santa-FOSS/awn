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

// fakeStrategy returns canned elements for format tests.
type fakeStrategy struct {
	elements []awn.Element
}

func (f fakeStrategy) Detect(_ *awn.Screen) []awn.Element { return f.elements }

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

func testScreen() *awn.Screen {
	cells := make([][]awn.Cell, 2)
	for r := range cells {
		cells[r] = make([]awn.Cell, 10)
		for c := range cells[r] {
			cells[r][c] = awn.Cell{Char: ' ', FG: awn.DefaultColor, BG: awn.DefaultColor}
		}
	}
	for i, ch := range "hello" {
		cells[0][i].Char = ch
	}
	return &awn.Screen{Rows: 2, Cols: 10, Cells: cells}
}

func TestBuildScreenResponse_DefaultFormat_ReturnsLinesNoElements(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "", nil)
	if resp.Lines == nil {
		t.Fatal("expected lines for default format")
	}
	if resp.Elements != nil {
		t.Fatal("expected no elements for default format")
	}
	if resp.State != "" {
		t.Fatalf("expected empty state for text format, got %q", resp.State)
	}
}

func TestBuildScreenResponse_StructuredFormat_IncludesState(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "structured", nil)
	if resp.State != "idle" {
		t.Fatalf("expected idle state, got %q", resp.State)
	}
}

func TestBuildScreenResponse_FullFormat_IncludesState(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "full", nil)
	if resp.State != "idle" {
		t.Fatalf("expected idle state, got %q", resp.State)
	}
}

func TestBuildScreenResponse_TextFormat_ReturnsLinesNoElements(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "text", nil)
	if resp.Lines == nil {
		t.Fatal("expected lines for text format")
	}
	if resp.Elements != nil {
		t.Fatal("expected no elements for text format")
	}
}

func TestBuildScreenResponse_StructuredFormat_ReturnsElementsNoLines(t *testing.T) {
	scr := testScreen()
	elems := []awn.Element{{Type: "button", Label: "OK"}}
	resp := buildScreenResponse(scr, "structured", elems)
	if resp.Lines != nil {
		t.Fatal("expected no lines for structured format")
	}
	if len(resp.Elements) != 1 || resp.Elements[0].Label != "OK" {
		t.Fatalf("expected 1 element with label OK, got %v", resp.Elements)
	}
}

func TestBuildScreenResponse_FullFormat_ReturnsBoth(t *testing.T) {
	scr := testScreen()
	elems := []awn.Element{{Type: "button", Label: "Save"}}
	resp := buildScreenResponse(scr, "full", elems)
	if resp.Lines == nil {
		t.Fatal("expected lines for full format")
	}
	if len(resp.Elements) != 1 || resp.Elements[0].Label != "Save" {
		t.Fatalf("expected 1 element with label Save, got %v", resp.Elements)
	}
}

func TestInferState_CursorAtPrompt_ReturnsWaitingForInput(t *testing.T) {
	scr := testScreen()
	// Put a "$ " prompt before cursor position
	for i, ch := range "$ " {
		scr.Cells[1][i].Char = ch
	}
	scr.Cursor = awn.Position{Row: 1, Col: 2}
	state := inferState(scr)
	if state != "waiting_for_input" {
		t.Fatalf("expected waiting_for_input, got %q", state)
	}
}

func TestInferState_EmptyScreen_ReturnsIdle(t *testing.T) {
	scr := testScreen()
	scr.Cursor = awn.Position{Row: 0, Col: 0}
	state := inferState(scr)
	if state != "idle" {
		t.Fatalf("expected idle, got %q", state)
	}
}

func TestInferState_CursorMidContent_ReturnsActive(t *testing.T) {
	scr := testScreen()
	// "hello" on row 0, cursor in the middle of content
	scr.Cursor = awn.Position{Row: 0, Col: 3}
	state := inferState(scr)
	if state != "active" {
		t.Fatalf("expected active, got %q", state)
	}
}

func TestInferState_ColonPrompt_ReturnsWaitingForInput(t *testing.T) {
	scr := testScreen()
	// vim-style ":" prompt
	scr.Cells[1][0].Char = ':'
	scr.Cursor = awn.Position{Row: 1, Col: 1}
	state := inferState(scr)
	if state != "waiting_for_input" {
		t.Fatalf("expected waiting_for_input, got %q", state)
	}
}
