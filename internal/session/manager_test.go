package session

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// pipePTY is a fake PTYStarter that returns a pipe pair instead of a real PTY.
// The write end is stored so tests can inject data; the read end is the "ptmx".
type pipePTY struct {
	W *os.File // tests write here
}

func newPipePTY() (*pipePTY, error) {
	return &pipePTY{}, nil
}

func (p *pipePTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.W = w
	// We don't actually start the command — the test controls all I/O.
	return r, nil
}

// TestDefaultConstants verifies exported row/col defaults.
func TestDefaultConstants(t *testing.T) {
	if DefaultRows != 24 {
		t.Errorf("DefaultRows = %d, want 24", DefaultRows)
	}
	if DefaultCols != 80 {
		t.Errorf("DefaultCols = %d, want 80", DefaultCols)
	}
}

// TestNewSession_CorrectDimensions verifies a new session screenshot has the right shape.
func TestNewSession_CorrectDimensions(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 10})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := m.Screenshot(id)
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if snap.Rows != 5 {
		t.Errorf("got %d rows, want 5", snap.Rows)
	}
	if snap.Cols != 10 {
		t.Errorf("got %d cols, want 10", snap.Cols)
	}
	if len(snap.Lines) != 5 {
		t.Errorf("got %d lines, want 5", len(snap.Lines))
	}
}

// TestNewSession_EmptyScreen verifies a fresh session screenshot has empty lines.
func TestNewSession_EmptyScreen(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 3, Cols: 4})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := m.Screenshot(id)
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	for i, line := range snap.Lines {
		if line != "" {
			t.Errorf("line %d = %q, want empty", i, line)
		}
	}
}

// TestList_Empty verifies a new manager has no sessions.
func TestList_Empty(t *testing.T) {
	m := NewManager()
	ids := m.List()
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

// TestGet_NotFound verifies that looking up a missing ID returns an error.
func TestGet_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.get("no-such-id")
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
}

// TestCreateAndScreenshot creates a session with the fake PTY, writes to it,
// and verifies the screenshot contains the written content.
func TestCreateAndScreenshot(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write a line to the fake PTY.
	_, err = io.WriteString(p.W, "hello world\r\n")
	if err != nil {
		t.Fatalf("write to pipe: %v", err)
	}

	// Give readLoop time to process.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, err := m.Screenshot(id)
		if err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		for _, line := range s.Lines {
			if strings.Contains(line, "hello world") {
				return // success
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for 'hello world' to appear in screenshot")
}

// TestClose_Idempotent verifies that closing a session twice does not panic.
func TestClose_Idempotent(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.Close(id); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should return an error (session not found), not panic.
	err = m.Close(id)
	if err == nil {
		t.Fatal("expected error on second Close, got nil")
	}
}

// TestWaitForText_FindsText writes text to the fake PTY and verifies
// WaitForText returns nil when the text appears.
func TestWaitForText_FindsText(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 40})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write in a separate goroutine after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		io.WriteString(p.W, "magic_token\r\n")
	}()

	err = m.WaitForText(id, "magic_token", 3*time.Second)
	if err != nil {
		t.Fatalf("WaitForText: %v", err)
	}
}

// TestWaitForText_Timeout verifies that WaitForText returns an error when
// the text never appears within the timeout.
func TestWaitForText_Timeout(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 40})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = m.WaitForText(id, "never_appears", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

// TestANSI_CursorMovement verifies that ANSI cursor-move escape sequences
// position text correctly instead of appearing as raw escape codes.
func TestANSI_CursorMovement(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// ESC[3;5H moves cursor to row 3, col 5 (1-based), then write "XY"
	_, err = io.WriteString(p.W, "\x1b[3;5HXY")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		// Row 2 (0-based) should contain "XY" starting at col 4 (0-based)
		if len(snap.Lines) > 2 && strings.Contains(snap.Lines[2], "XY") {
			// Verify no raw escape codes appear anywhere
			for _, line := range snap.Lines {
				if strings.Contains(line, "\x1b") {
					t.Fatalf("raw escape code found in screenshot: %q", line)
				}
			}
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out: expected 'XY' at row 2 from ANSI cursor move")
}

// TestANSI_CursorPositionTracking verifies that cursor position in the
// screenshot reflects ANSI cursor positioning commands.
func TestANSI_CursorPositionTracking(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 10, Cols: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move cursor to row 5, col 8 (1-based) then write "Z"
	// After writing "Z", cursor should be at row 5, col 9 (1-based) = row 4, col 8 (0-based)
	_, err = io.WriteString(p.W, "\x1b[5;8HZ")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		if len(snap.Lines) > 4 && strings.Contains(snap.Lines[4], "Z") {
			// Cursor should be at (row=4, col=8) 0-based after writing "Z"
			if snap.Cursor.Row != 4 {
				t.Errorf("cursor row = %d, want 4", snap.Cursor.Row)
			}
			if snap.Cursor.Col != 8 {
				t.Errorf("cursor col = %d, want 8", snap.Cursor.Col)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out: expected 'Z' at row 4 from ANSI cursor positioning")
}

// TestANSI_AlternateScreenBuffer verifies that switching to the alternate
// screen buffer and writing content is captured correctly in screenshots.
func TestANSI_AlternateScreenBuffer(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	id, err := m.Create(Config{Command: "true", Rows: 5, Cols: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write "MAIN" on main screen
	_, err = io.WriteString(p.W, "MAIN")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Wait for "MAIN" to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap, _ := m.Screenshot(id)
		if strings.Contains(snap.Lines[0], "MAIN") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Switch to alternate screen buffer (ESC[?1049h) and write "ALT"
	_, err = io.WriteString(p.W, "\x1b[?1049h\x1b[1;1HALT")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		if strings.Contains(snap.Lines[0], "ALT") {
			// "MAIN" should NOT be visible on the alternate screen
			for _, line := range snap.Lines {
				if strings.Contains(line, "MAIN") {
					t.Fatal("MAIN should not be visible on alternate screen")
				}
			}
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out: expected 'ALT' on alternate screen buffer")
}

// TestANSI_ScrollUp verifies that writing more lines than the screen height
// causes the screen to scroll, pushing earlier content up and off-screen.
func TestANSI_ScrollUp(t *testing.T) {
	p := &pipePTY{}
	m := NewManagerWithPTY(p)

	rows := 3
	id, err := m.Create(Config{Command: "true", Rows: rows, Cols: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write 5 lines to a 3-row terminal — first 2 lines should scroll off
	_, err = io.WriteString(p.W, "AAA\r\nBBB\r\nCCC\r\nDDD\r\nEEE")
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			t.Fatalf("Screenshot: %v", err)
		}
		// After scrolling, last line should have "EEE"
		if strings.Contains(snap.Lines[rows-1], "EEE") {
			// "AAA" should have scrolled off
			for _, line := range snap.Lines {
				if strings.Contains(line, "AAA") {
					t.Fatal("AAA should have scrolled off screen")
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out: expected 'EEE' on last line after scrolling")
}
