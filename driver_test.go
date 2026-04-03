package awn

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
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

// newTestSession creates a Driver with a pipePTY and starts a session.
// It returns the pipe, driver, and session. It calls t.Fatal on error.
func newTestSession(t *testing.T) (*pipePTY, *Driver, *Session) {
	t.Helper()
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))
	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	return p, d, s
}

func TestNewDriver_ListsZeroSessions(t *testing.T) {
	d := NewDriver()
	if got := d.List(); len(got) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(got))
	}
}

func TestDriver_Session_AppearsInList(t *testing.T) {
	_, d, s := newTestSession(t)
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
	p, _, s := newTestSession(t)

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
	p, _, s := newTestSession(t)

	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = p.W.WriteString("ready")
	}()

	if err := s.WaitForText("ready", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}
}

func TestSession_WaitForText_Timeout(t *testing.T) {
	_, _, s := newTestSession(t)

	err := s.WaitForText("never", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSession_WaitForStable_ReturnsWhenStable(t *testing.T) {
	p, _, s := newTestSession(t)

	_, _ = p.W.WriteString("fixed content")
	if err := s.WaitForText("fixed content", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	if err := s.WaitForStable(50*time.Millisecond, time.Second); err != nil {
		t.Fatalf("WaitForStable: %v", err)
	}
}

func TestSession_WaitForStable_TimeoutWhenChanging(t *testing.T) {
	p, _, s := newTestSession(t)

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

	err := s.WaitForStable(200*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSession_Screen_ReturnsCorrectDimensions(t *testing.T) {
	_, d, s := newTestSession(t)
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

func TestSession_Resize_UpdatesScreenDimensions(t *testing.T) {
	s := &Session{
		term:     vt10x.New(vt10x.WithSize(80, 24)),
		cfg:      Config{Rows: 24, Cols: 80},
		restored: true,
		snapshot: &Screen{Rows: 24, Cols: 80, Cells: make([][]Cell, 24)},
	}

	if err := s.Resize(40, 100); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	screen := s.Screen()
	if screen.Rows != 40 {
		t.Fatalf("Screen().Rows = %d, want %d", screen.Rows, 40)
	}
	if screen.Cols != 100 {
		t.Fatalf("Screen().Cols = %d, want %d", screen.Cols, 100)
	}
}

func TestSession_Resize_LiveSessionUsesPTYResizer(t *testing.T) {
	called := false
	s := &Session{
		ptmx: os.NewFile(0, "/dev/null"),
		term: vt10x.New(vt10x.WithSize(80, 24)),
		cfg:  Config{Rows: 24, Cols: 80},
		resizePTY: func(_ *os.File, ws *pty.Winsize) error {
			called = true
			if ws.Rows != 40 || ws.Cols != 100 {
				t.Fatalf("winsize = %#v, want rows=40 cols=100", ws)
			}
			return nil
		},
	}

	if err := s.Resize(40, 100); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if !called {
		t.Fatal("expected PTY resizer to be called")
	}
}

func TestSession_Screen_CapturesCharacterContent(t *testing.T) {
	p, d, s := newTestSession(t)
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

func TestSession_Screen_CapturesAttributes(t *testing.T) {
	tests := []struct {
		name   string
		escape string
		char   rune
		check  func(t *testing.T, cell Cell)
	}{
		{"Reverse", "\x1b[7mhi\x1b[0m", 'h', func(t *testing.T, cell Cell) {
			if cell.Attrs&AttrReverse == 0 {
				t.Errorf("expected AttrReverse, got attrs=%d", cell.Attrs)
			}
		}},
		{"Bold", "\x1b[1mB\x1b[0m", 'B', func(t *testing.T, cell Cell) {
			if cell.Attrs&AttrBold == 0 {
				t.Errorf("expected AttrBold, got attrs=%d", cell.Attrs)
			}
		}},
		{"Underline", "\x1b[4mU\x1b[0m", 'U', func(t *testing.T, cell Cell) {
			if cell.Attrs&AttrUnderline == 0 {
				t.Errorf("expected AttrUnderline, got attrs=%d", cell.Attrs)
			}
		}},
		{"FGColor", "\x1b[31mr\x1b[0m", 'r', func(t *testing.T, cell Cell) {
			if cell.FG == DefaultColor {
				t.Error("expected non-default FG for red text")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _, s := newTestSession(t)
			_, _ = p.W.WriteString(tt.escape)
			if err := s.WaitForText(string(tt.char), time.Second); err != nil {
				t.Fatalf("WaitForText: %v", err)
			}
			cell := s.Screen().Cells[0][0]
			if cell.Char != tt.char {
				t.Fatalf("char = %q, want %q", cell.Char, tt.char)
			}
			tt.check(t, cell)
		})
	}
}

func TestSession_Close_RemovesFromDriver(t *testing.T) {
	_, d, s := newTestSession(t)

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

func TestSession_SendMouseClick_WritesSGRSequence(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	d := NewDriver(WithPTY(&bidirPTY{ptmx: w}))
	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	if err := s.SendMouseClick(4, 7, 0); err != nil {
		t.Fatalf("SendMouseClick: %v", err)
	}

	buf := make([]byte, 32)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := string(buf[:n]), "\x1b[<0;8;5M\x1b[<0;8;5m"; got != want {
		t.Fatalf("mouse click = %q, want %q", got, want)
	}
}

func TestSession_SendMouseMove_WritesSGRSequence(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	d := NewDriver(WithPTY(&bidirPTY{ptmx: w}))
	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	if err := s.SendMouseMove(2, 3); err != nil {
		t.Fatalf("SendMouseMove: %v", err)
	}

	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := string(buf[:n]), "\x1b[<35;4;3M"; got != want {
		t.Fatalf("mouse move = %q, want %q", got, want)
	}
}

func TestSession_Scrollback_ReturnsRecentLines(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.SessionWithConfig(Config{Command: "true", Scrollback: 2})
	if err != nil {
		t.Fatalf("SessionWithConfig: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	_, _ = p.W.WriteString("one\ntwo\nthree\n")
	if err := s.WaitForText("three", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	if got := s.Scrollback(10); len(got) != 2 || got[0] != "two" || got[1] != "three" {
		t.Fatalf("Scrollback = %#v, want [\"two\", \"three\"]", got)
	}
}

func TestSession_RecordAsciicast_WritesV2File(t *testing.T) {
	p, d, s := newTestSession(t)
	defer d.Close(s.ID) //nolint:errcheck

	_, _ = p.W.WriteString("hello")
	if err := s.WaitForText("hello", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	path := filepath.Join(t.TempDir(), "session.cast")
	if err := s.RecordAsciicast(path); err != nil {
		t.Fatalf("RecordAsciicast: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and event lines, got %q", string(data))
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header.Version != 2 {
		t.Fatalf("header version = %d, want 2", header.Version)
	}
	if !strings.Contains(lines[1], "hello") {
		t.Fatalf("expected output event to contain hello, got %q", lines[1])
	}
}

func TestDriver_RestoresPersistedSessions(t *testing.T) {
	dir := t.TempDir()
	p := &pipePTY{}
	d := NewDriver(WithPTY(p), WithPersistenceDir(dir))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("persisted output\n")
	if err := s.WaitForText("persisted output", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	restored := NewDriver(WithPersistenceDir(dir))
	got := restored.Get(s.ID)
	if got == nil {
		t.Fatal("expected restored session, got nil")
	}
	if !strings.Contains(got.Text(), "persisted output") {
		t.Fatalf("restored text = %q, want persisted output", got.Text())
	}
	if len(got.Scrollback(10)) == 0 {
		t.Fatal("expected restored scrollback")
	}
}

func TestDriver_Close_RemovesPersistedSession(t *testing.T) {
	dir := t.TempDir()
	p := &pipePTY{}
	d := NewDriver(WithPTY(p), WithPersistenceDir(dir))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	_, _ = p.W.WriteString("closing output\n")
	if err := s.WaitForText("closing output", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	if err := d.Close(s.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restored := NewDriver(WithPersistenceDir(dir))
	if got := restored.Get(s.ID); got != nil {
		t.Fatalf("expected closed session to stay deleted, got %#v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, s.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected persisted session file removed, stat err = %v", err)
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
	_, d, s := newTestSession(t)
	defer d.Close(s.ID) //nolint:errcheck

	if !s.ContainsText("") {
		t.Fatal("expected ContainsText('') to return true")
	}
}

func TestDriver_Get_ReturnsSession(t *testing.T) {
	_, d, s := newTestSession(t)
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
	// Create a second session in the same driver. pipePTY.Start creates a fresh
	// pipe per call, overwriting p.W, which is fine — we only need both
	// sessions registered in d.
	_, err = d.Session("true")
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
	_, d, s := newTestSession(t)
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
	_, d, s := newTestSession(t)
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
	_, d, s := newTestSession(t)
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
	_, d, s := newTestSession(t)
	defer d.Close(s.ID) //nolint:errcheck

	strategy := &mockStrategy{elements: []Element{
		{Type: "button", Label: "OK"},
	}}

	_, err := s.FindOne(strategy, ByLabel("Cancel"))
	if err == nil {
		t.Fatal("expected error when no element matches, got nil")
	}
}
