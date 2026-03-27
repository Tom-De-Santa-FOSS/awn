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

// TestMakeBuffer_CorrectDimensions verifies makeBuffer produces the right shape.
func TestMakeBuffer_CorrectDimensions(t *testing.T) {
	rows, cols := 5, 10
	buf := makeBuffer(rows, cols)
	if len(buf) != rows {
		t.Fatalf("got %d rows, want %d", len(buf), rows)
	}
	for i, row := range buf {
		if len(row) != cols {
			t.Fatalf("row %d: got %d cols, want %d", i, len(row), cols)
		}
	}
}

// TestMakeBuffer_AllSpaces verifies every cell starts as a space.
func TestMakeBuffer_AllSpaces(t *testing.T) {
	buf := makeBuffer(3, 4)
	for r, row := range buf {
		for c, ch := range row {
			if ch != ' ' {
				t.Errorf("buf[%d][%d] = %q, want ' '", r, c, ch)
			}
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
