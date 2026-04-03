package rpc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/tom/awn"
)

// stubStrategy is a no-op strategy for handler tests.
type stubStrategy struct{}

func (stubStrategy) Detect(screen *awn.Screen) []awn.Element { return nil }

func newTestHandler() *Handler {
	d := awn.NewDriver()
	return NewHandler(d, stubStrategy{})
}

type pipePTY struct {
	W *os.File
}

func (p *pipePTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.W = w
	return r, nil
}

type bidirPTY struct {
	ptmx *os.File
}

func (b *bidirPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	return b.ptmx, nil
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

func TestDispatch_Ping(t *testing.T) {
	h := newTestHandler()
	result, err := h.Dispatch("ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*PingResponse)
	if !ok {
		t.Fatalf("expected *PingResponse, got %T", result)
	}
	if resp.Status != "ok" {
		t.Fatalf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestDispatch_MethodNotFound_ForNonexistentSession(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		params          string
		rejectMethodErr bool
		rejectParamsErr bool
	}{
		{name: "screenshot", method: "screenshot", params: `{"id":"nonexistent"}`},
		{name: "detect", method: "detect", params: `{"id":"nonexistent"}`},
		{name: "input", method: "input", params: `{"id":"nonexistent","data":"x"}`},
		{name: "wait_for_text", method: "wait_for_text", params: `{"id":"nonexistent","text":"x"}`},
		{name: "wait_for_stable", method: "wait_for_stable", params: `{"id":"nonexistent"}`},
		{name: "close", method: "close", params: `{"id":"nonexistent"}`},
		{name: "resize", method: "resize", params: `{"id":"nonexistent","rows":40,"cols":100}`, rejectMethodErr: true, rejectParamsErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler()
			_, err := h.Dispatch(tt.method, json.RawMessage(tt.params))
			if err == nil {
				t.Fatalf("expected error for nonexistent session, got nil")
			}
			if tt.rejectMethodErr && strings.Contains(err.Error(), "method not found") {
				t.Fatalf("expected %s route to exist, got: %v", tt.method, err)
			}
			if tt.rejectParamsErr && strings.Contains(err.Error(), "invalid params") {
				t.Fatalf("expected %s params to be accepted, got: %v", tt.method, err)
			}
		})
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
	resp := buildScreenResponse(scr, "", nil, nil, nil, "")
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

func TestBuildScreenResponse_DefaultFormat_IncludesScreenHash(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "", nil, nil, nil, "")
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(scr.Text())))
	if resp.Hash != want {
		t.Fatalf("Hash = %q, want %q", resp.Hash, want)
	}
}

func TestBuildScreenResponse_StructuredFormat_IncludesState(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "structured", nil, nil, nil, "")
	if resp.State != "idle" {
		t.Fatalf("expected idle state, got %q", resp.State)
	}
}

func TestBuildScreenResponse_FullFormat_IncludesState(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "full", nil, nil, nil, "")
	if resp.State != "idle" {
		t.Fatalf("expected idle state, got %q", resp.State)
	}
}

func TestBuildScreenResponse_TextFormat_ReturnsLinesNoElements(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "text", nil, nil, nil, "")
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
	resp := buildScreenResponse(scr, "structured", elems, nil, nil, "")
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
	resp := buildScreenResponse(scr, "full", elems, nil, nil, "")
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

func TestScreenResponse_JSON_TextFormat_OmitsElementsAndState(t *testing.T) {
	resp := ScreenResponse{
		Rows:   2,
		Cols:   10,
		Lines:  []string{"hello", ""},
		Cursor: awn.Position{Row: 0, Col: 5},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "elements") {
		t.Fatalf("text response should omit elements, got: %s", s)
	}
	if strings.Contains(s, "state") {
		t.Fatalf("text response should omit state, got: %s", s)
	}
}

func TestScreenResponse_JSON_StructuredFormat_OmitsLines(t *testing.T) {
	resp := ScreenResponse{
		Rows:     2,
		Cols:     10,
		Cursor:   awn.Position{Row: 0, Col: 0},
		Elements: []awn.Element{{Type: "button", Label: "OK"}},
		State:    "idle",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "lines") {
		t.Fatalf("structured response should omit lines, got: %s", s)
	}
	if !strings.Contains(s, `"elements"`) {
		t.Fatalf("structured response should include elements, got: %s", s)
	}
	if !strings.Contains(s, `"state"`) {
		t.Fatalf("structured response should include state, got: %s", s)
	}
}

func TestDispatch_ScreenshotWithFormat_AcceptsFormatParam(t *testing.T) {
	h := newTestHandler()
	// All three formats should be accepted (session-not-found error, not invalid-params).
	for _, format := range []string{"text", "structured", "full", "diff"} {
		params := fmt.Sprintf(`{"id":"nonexistent","format":%q}`, format)
		_, err := h.Dispatch("screenshot", json.RawMessage(params))
		if err == nil {
			t.Fatalf("format=%s: expected error, got nil", format)
		}
		if strings.Contains(err.Error(), "invalid params") {
			t.Fatalf("format=%s: rejected format param: %v", format, err)
		}
	}
}

func TestBuildScreenResponse_WithScrollback_IncludesHistory(t *testing.T) {
	scr := testScreen()
	resp := buildScreenResponse(scr, "", nil, []string{"one", "two"}, nil, "")
	if len(resp.History) != 2 || resp.History[0] != "one" || resp.History[1] != "two" {
		t.Fatalf("History = %#v, want [\"one\", \"two\"]", resp.History)
	}
}

func TestBuildScreenResponse_DiffFormat_OmitsLinesAndIncludesChanges(t *testing.T) {
	scr := testScreen()
	changes := []ScreenChange{{Row: 1, Lines: []string{"updated"}}}
	resp := buildScreenResponse(scr, "diff", nil, nil, changes, "base")
	if resp.Lines != nil {
		t.Fatalf("expected diff response to omit lines, got %#v", resp.Lines)
	}
	if len(resp.Changes) != 1 || resp.Changes[0].Row != 1 {
		t.Fatalf("Changes = %#v, want row 1 change", resp.Changes)
	}
	if resp.BaseHash != "base" {
		t.Fatalf("BaseHash = %q, want %q", resp.BaseHash, "base")
	}
}

func TestHandler_Screenshot_WithScrollback_IncludesHistory(t *testing.T) {
	p := &pipePTY{}
	d := awn.NewDriver(awn.WithPTY(p))
	h := NewHandler(d, stubStrategy{})

	s, err := d.SessionWithConfig(awn.Config{Command: "true", Scrollback: 2})
	if err != nil {
		t.Fatalf("SessionWithConfig: %v", err)
	}

	_, _ = p.W.WriteString("one\ntwo\nthree\n")
	if err := s.WaitForText("three", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	resp, err := h.Screenshot(ScreenshotRequest{ID: s.ID, Scrollback: 2})
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if len(resp.History) != 2 || resp.History[0] != "two" || resp.History[1] != "three" {
		t.Fatalf("History = %#v, want [\"two\", \"three\"]", resp.History)
	}
}

func TestHandler_Screenshot_DiffFormat_ReturnsChangedRows(t *testing.T) {
	p := &pipePTY{}
	d := awn.NewDriver(awn.WithPTY(p))
	h := NewHandler(d, stubStrategy{})

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("before")
	if err := s.WaitForText("before", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}
	if _, err := h.Screenshot(ScreenshotRequest{ID: s.ID, Format: "diff"}); err != nil {
		t.Fatalf("first Screenshot: %v", err)
	}

	_, _ = p.W.WriteString("\rafter ")
	if err := s.WaitForText("after", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	resp, err := h.Screenshot(ScreenshotRequest{ID: s.ID, Format: "diff"})
	if err != nil {
		t.Fatalf("second Screenshot: %v", err)
	}
	if resp.Lines != nil {
		t.Fatalf("expected diff response to omit lines, got %#v", resp.Lines)
	}
	if len(resp.Changes) == 0 {
		t.Fatal("expected diff changes, got none")
	}
}

func TestDispatch_MouseClick_WritesMouseSequence(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	d := awn.NewDriver(awn.WithPTY(&bidirPTY{ptmx: w}))
	h := NewHandler(d, stubStrategy{})
	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, err = h.Dispatch("mouse_click", json.RawMessage(fmt.Sprintf(`{"id":%q,"row":1,"col":2,"button":0}`, s.ID)))
	if err == nil {
		buf := make([]byte, 32)
		n, readErr := r.Read(buf)
		if readErr != nil {
			t.Fatalf("Read: %v", readErr)
		}
		if got, want := string(buf[:n]), "\x1b[<0;3;2M\x1b[<0;3;2m"; got != want {
			t.Fatalf("mouse click = %q, want %q", got, want)
		}
		return
	}
	t.Fatalf("Dispatch: %v", err)
}

func TestDispatch_Record_WritesCastFile(t *testing.T) {
	p := &pipePTY{}
	d := awn.NewDriver(awn.WithPTY(p))
	h := NewHandler(d, stubStrategy{})

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	_, _ = p.W.WriteString("hello")
	if err := s.WaitForText("hello", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	path := filepath.Join(t.TempDir(), "session.cast")
	_, err = h.Dispatch("record", json.RawMessage(fmt.Sprintf(`{"id":%q,"path":%q}`, s.ID, path)))
	if err == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		if !strings.Contains(string(data), "hello") {
			t.Fatalf("expected cast output to include hello, got %q", string(data))
		}
		return
	}
	t.Fatalf("Dispatch: %v", err)
}

func TestInferState_CursorRowOutOfBounds_ReturnsIdle(t *testing.T) {
	scr := testScreen()
	scr.Cursor = awn.Position{Row: 5, Col: 3}
	state := inferState(scr)
	if state != "idle" {
		t.Fatalf("expected idle, got %q", state)
	}
}

func TestInferState_CursorColOutOfBounds_ReturnsIdle(t *testing.T) {
	scr := testScreen()
	scr.Cursor = awn.Position{Row: 0, Col: 99}
	state := inferState(scr)
	if state != "idle" {
		t.Fatalf("expected idle, got %q", state)
	}
}

func TestInferState_HashPrompt_ReturnsWaitingForInput(t *testing.T) {
	scr := testScreen()
	for i, ch := range "# " {
		scr.Cells[1][i].Char = ch
	}
	scr.Cursor = awn.Position{Row: 1, Col: 2}
	state := inferState(scr)
	if state != "waiting_for_input" {
		t.Fatalf("expected waiting_for_input, got %q", state)
	}
}

func TestInferState_PercentPrompt_ReturnsWaitingForInput(t *testing.T) {
	scr := testScreen()
	for i, ch := range "% " {
		scr.Cells[1][i].Char = ch
	}
	scr.Cursor = awn.Position{Row: 1, Col: 2}
	state := inferState(scr)
	if state != "waiting_for_input" {
		t.Fatalf("expected waiting_for_input, got %q", state)
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
