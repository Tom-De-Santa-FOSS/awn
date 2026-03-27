package session

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/tom/awn/internal/screen"
)

// Manager handles multiple concurrent TUI sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Create spawns a new TUI session in a PTY.
func (m *Manager) Create(cfg Config) (string, error) {
	cfg.defaults()

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TERM=xterm-256color"),
		fmt.Sprintf("COLUMNS=%d", cfg.Cols),
		fmt.Sprintf("LINES=%d", cfg.Rows),
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(cfg.Rows),
		Cols: uint16(cfg.Cols),
	})
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	id := uuid.New().String()[:8]
	sess := &Session{
		ID:      id,
		Cmd:     cmd,
		ptmx:    ptmx,
		rows:    cfg.Rows,
		cols:    cfg.Cols,
		buf:     makeBuffer(cfg.Rows, cfg.Cols),
		done:    make(chan struct{}),
		created: time.Now(),
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	go sess.readLoop()

	return id, nil
}

// Screenshot captures the current screen state of a session.
func (m *Manager) Screenshot(id string) (*screen.Snapshot, error) {
	sess, err := m.get(id)
	if err != nil {
		return nil, err
	}

	sess.mu.RLock()
	defer sess.mu.RUnlock()

	snap := &screen.Snapshot{
		Rows:  sess.rows,
		Cols:  sess.cols,
		Lines: make([]string, sess.rows),
	}

	for i, row := range sess.buf {
		snap.Lines[i] = strings.TrimRight(string(row), " \x00")
	}

	return snap, nil
}

// Input sends keystrokes to a session.
func (m *Manager) Input(id string, data string) error {
	sess, err := m.get(id)
	if err != nil {
		return err
	}

	_, err = sess.ptmx.WriteString(data)
	return err
}

// WaitForText polls until the given text appears on screen or timeout.
func (m *Manager) WaitForText(id string, text string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			return err
		}
		for _, line := range snap.Lines {
			if strings.Contains(line, text) {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q after %s", text, timeout)
}

// WaitForStable polls until the screen stops changing.
func (m *Manager) WaitForStable(id string, stable time.Duration, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastSnap string
	stableSince := time.Now()

	for time.Now().Before(deadline) {
		snap, err := m.Screenshot(id)
		if err != nil {
			return err
		}
		current := snap.Text()
		if current == lastSnap {
			if time.Since(stableSince) >= stable {
				return nil
			}
		} else {
			lastSnap = current
			stableSince = time.Now()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for screen stability after %s", timeout)
}

// Close terminates a session.
func (m *Manager) Close(id string) error {
	sess, err := m.get(id)
	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	sess.ptmx.Close()
	if sess.Cmd.Process != nil {
		sess.Cmd.Process.Kill()
	}
	close(sess.done)
	return nil
}

// List returns all active session IDs.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return sess, nil
}

// readLoop reads PTY output and updates the screen buffer.
// This is a simplified line-based parser. For full VT100 emulation,
// swap in bubbleterm or go-vte.
func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	row, col := 0, 0

	for {
		select {
		case <-s.done:
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if err != nil {
			return
		}

		s.mu.Lock()
		for _, b := range buf[:n] {
			switch b {
			case '\n':
				row++
				col = 0
				if row >= s.rows {
					// Scroll up
					copy(s.buf, s.buf[1:])
					s.buf[s.rows-1] = make([]rune, s.cols)
					row = s.rows - 1
				}
			case '\r':
				col = 0
			default:
				if row < s.rows && col < s.cols {
					s.buf[row][col] = rune(b)
					col++
				}
			}
		}
		s.mu.Unlock()
	}
}

func makeBuffer(rows, cols int) [][]rune {
	buf := make([][]rune, rows)
	for i := range buf {
		buf[i] = make([]rune, cols)
		for j := range buf[i] {
			buf[i][j] = ' '
		}
	}
	return buf
}
