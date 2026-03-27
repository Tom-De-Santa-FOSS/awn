package awn

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// pipePTY is a fake PTYStarter that returns a pipe instead of a real PTY.
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

func TestNewDriver_ListsZeroSessions(t *testing.T) {
	d := NewDriver()
	if got := d.List(); len(got) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(got))
	}
}

func TestDriver_Session_AppearsInList(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if s.ID == "" {
		t.Fatal("session ID is empty")
	}

	ids := d.List()
	if len(ids) != 1 {
		t.Fatalf("expected 1 session, got %d", len(ids))
	}
	if ids[0] != s.ID {
		t.Errorf("got %q, want %q", ids[0], s.ID)
	}
}

func TestSession_Text_ReturnsTerminalContent(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("hello world")
	// Give readLoop time to process.
	time.Sleep(20 * time.Millisecond)
	txt := s.Text()
	if !contains(txt, "hello world") {
		t.Fatalf("text %q does not contain 'hello world'", txt)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}

func TestSession_WaitForText_FindsText(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = p.W.WriteString("ready")
	}()

	if err := s.WaitForText("ready", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}
}

func TestSession_WaitForText_Timeout(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	err = s.WaitForText("never", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSession_Screen_ReturnsCorrectDimensions(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer s.Close()

	screen := s.Screen()
	if screen.Rows != DefaultRows {
		t.Fatalf("Screen().Rows = %d, want %d", screen.Rows, DefaultRows)
	}
	if screen.Cols != DefaultCols {
		t.Fatalf("Screen().Cols = %d, want %d", screen.Cols, DefaultCols)
	}
	if len(screen.Cells) != screen.Rows {
		t.Fatalf("len(Cells) = %d, want %d", len(screen.Cells), screen.Rows)
	}
	for i, row := range screen.Cells {
		if len(row) != screen.Cols {
			t.Fatalf("len(Cells[%d]) = %d, want %d", i, len(row), screen.Cols)
		}
	}
}

func TestSession_Close_RemovesFromDriver(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if err := d.Close(s.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if len(d.List()) != 0 {
		t.Fatal("expected 0 sessions after close")
	}
}

// mockStrategy is a test double that returns a fixed set of elements.
type mockStrategy struct {
	elements []Element
}

func (m *mockStrategy) Detect(screen *Screen) []Element {
	return m.elements
}

func TestSession_FindAll_ReturnsMockElements(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	want := []Element{
		{Type: "button", Label: "OK"},
		{Type: "button", Label: "Cancel"},
	}
	strategy := &mockStrategy{elements: want}

	got := s.FindAll(strategy)
	if len(got) != len(want) {
		t.Fatalf("FindAll returned %d elements, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e != want[i] {
			t.Errorf("element[%d] = %v, want %v", i, e, want[i])
		}
	}
}

func TestSession_FindOne_ReturnsFirstMatchingElement(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	elements := []Element{
		{Type: "button", Label: "OK"},
		{Type: "button", Label: "Cancel"},
	}
	strategy := &mockStrategy{elements: elements}

	got, err := s.FindOne(strategy, ByLabel("Cancel"))
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.Label != "Cancel" {
		t.Errorf("FindOne returned element with label %q, want %q", got.Label, "Cancel")
	}
}
