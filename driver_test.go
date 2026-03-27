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

// errorPTY always fails to start.
type errorPTY struct{}

func (errorPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	return nil, os.ErrPermission
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
	if err := s.WaitForText("hello world", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}
	txt := s.Text()
	if !strings.Contains(txt, "hello world") {
		t.Fatalf("text %q does not contain 'hello world'", txt)
	}
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

func TestSession_WaitForStable_ReturnsWhenStable(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("fixed content")
	if err := s.WaitForText("fixed content", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	if err := s.WaitForStable(50*time.Millisecond, time.Second); err != nil {
		t.Fatalf("WaitForStable: %v", err)
	}
}

func TestSession_WaitForStable_TimeoutWhenChanging(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	// Keep writing to prevent stability.
	stop := make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = p.W.WriteString(string(rune('a' + (i % 26))))
				i++
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
	defer close(stop)

	err = s.WaitForStable(200*time.Millisecond, 100*time.Millisecond)
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
	defer d.Close(s.ID) //nolint:errcheck

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

func TestSession_Screen_CapturesCharacterContent(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	_, _ = p.W.WriteString("hi")
	if err := s.WaitForText("hi", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	scr := s.Screen()
	if scr.Cells[0][0].Char != 'h' {
		t.Errorf("Cells[0][0].Char = %q, want 'h'", scr.Cells[0][0].Char)
	}
	if scr.Cells[0][1].Char != 'i' {
		t.Errorf("Cells[0][1].Char = %q, want 'i'", scr.Cells[0][1].Char)
	}
}

func TestSession_Screen_CapturesReverseAttr(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("\x1b[7mhi\x1b[0m")
	if err := s.WaitForText("hi", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	scr := s.Screen()
	cell := scr.Cells[0][0]
	if cell.Char != 'h' {
		t.Fatalf("char = %q, want 'h'", cell.Char)
	}
	if cell.Attrs&AttrReverse == 0 {
		t.Errorf("expected AttrReverse, got attrs=%d", cell.Attrs)
	}
}

func TestSession_Screen_CapturesBoldAttr(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("\x1b[1mB\x1b[0m")
	if err := s.WaitForText("B", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	cell := s.Screen().Cells[0][0]
	if cell.Char != 'B' {
		t.Fatalf("char = %q, want 'B'", cell.Char)
	}
	if cell.Attrs&AttrBold == 0 {
		t.Errorf("expected AttrBold, got attrs=%d", cell.Attrs)
	}
}

func TestSession_Screen_CapturesUnderlineAttr(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("\x1b[4mU\x1b[0m")
	if err := s.WaitForText("U", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	cell := s.Screen().Cells[0][0]
	if cell.Char != 'U' {
		t.Fatalf("char = %q, want 'U'", cell.Char)
	}
	if cell.Attrs&AttrUnderline == 0 {
		t.Errorf("expected AttrUnderline, got attrs=%d", cell.Attrs)
	}
}

func TestSession_Screen_CapturesFGColor(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("\x1b[31mr\x1b[0m")
	if err := s.WaitForText("r", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	scr := s.Screen()
	cell := scr.Cells[0][0]
	if cell.Char != 'r' {
		t.Fatalf("char = %q, want 'r'", cell.Char)
	}
	if cell.FG == DefaultColor {
		t.Error("expected non-default FG for red text")
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

func TestSession_SendKeys_WritesToPTY(t *testing.T) {
	// Use a bidirectional pipe so SendKeys can write to ptmx.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	bp := &bidirPTY{ptmx: w}
	d := NewDriver(WithPTY(bp))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	if err := s.SendKeys("hello"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	// Verify data was written.
	buf := make([]byte, 5)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("got %q, want %q", string(buf[:n]), "hello")
	}
}

// bidirPTY returns a writable file as ptmx for SendKeys testing.
type bidirPTY struct {
	ptmx *os.File
}

func (b *bidirPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	return b.ptmx, nil
}

func TestSession_ContainsText_EmptyString(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	if !s.ContainsText("") {
		t.Fatal("expected ContainsText('') to return true")
	}
}

func TestDriver_Get_ReturnsSession(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	got := d.Get(s.ID)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != s.ID {
		t.Errorf("got ID %q, want %q", got.ID, s.ID)
	}
}

func TestDriver_Get_ReturnsNilForMissing(t *testing.T) {
	d := NewDriver()
	if got := d.Get("nonexistent"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestDriver_CloseAll_ClosesAllSessions(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	_, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	// Need a new pipe for the second session.
	p2 := &pipePTY{}
	d2 := NewDriver(WithPTY(p2))
	_, err = d2.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	d.CloseAll()
	if len(d.List()) != 0 {
		t.Fatal("expected 0 sessions after CloseAll")
	}
}

func TestDriver_SessionWithConfig_DefaultRowsCols(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.SessionWithConfig(Config{Command: "true"})
	if err != nil {
		t.Fatalf("SessionWithConfig: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	scr := s.Screen()
	if scr.Rows != DefaultRows {
		t.Errorf("Rows = %d, want %d", scr.Rows, DefaultRows)
	}
	if scr.Cols != DefaultCols {
		t.Errorf("Cols = %d, want %d", scr.Cols, DefaultCols)
	}
}

func TestDriver_SessionWithConfig_PTYStartError(t *testing.T) {
	d := NewDriver(WithPTY(errorPTY{}))

	_, err := d.Session("true")
	if err == nil {
		t.Fatal("expected error from PTY start failure")
	}
}

func TestSession_Close_NilProcess(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	// pipePTY doesn't start a real process, so cmd.Process is nil.
	// Close should not panic.
	if err := d.Close(s.ID); err != nil {
		t.Fatalf("Close: %v", err)
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

func TestSession_FindOne_ErrorWhenNoElementMatches(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	strategy := &mockStrategy{elements: []Element{
		{Type: "button", Label: "OK"},
	}}

	_, err = s.FindOne(strategy, ByLabel("Cancel"))
	if err == nil {
		t.Fatal("expected error when no element matches, got nil")
	}
}
