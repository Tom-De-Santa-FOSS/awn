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

// realPTY is the production PTYStarter that uses creack/pty.
type realPTY struct{}

func (realPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	return pty.StartWithSize(cmd, ws)
}

// Manager handles multiple concurrent TUI sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	pty      PTYStarter
}

// NewManager creates a session manager with the real PTY backend.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		pty:      realPTY{},
	}
}

// NewManagerWithPTY creates a session manager with an injected PTY backend.
func NewManagerWithPTY(p PTYStarter) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		pty:      p,
	}
}

// Create spawns a new TUI session in a PTY.
func (m *Manager) Create(cfg Config) (string, error) {
	cfg.defaults()

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		fmt.Sprintf("COLUMNS=%d", cfg.Cols),
		fmt.Sprintf("LINES=%d", cfg.Rows),
	)

	ptmx, err := m.pty.Start(cmd, &pty.Winsize{
		Rows: uint16(cfg.Rows),
		Cols: uint16(cfg.Cols),
	})
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	id := uuid.New().String()
	sess := &Session{
		ID:      id,
		Cmd:     cmd,
		ptmx:    ptmx,
		rows:    cfg.Rows,
		cols:    cfg.Cols,
		buf:     makeBuffer(cfg.Rows, cfg.Cols),
		done:    make(chan struct{}),
		updated: make(chan struct{}, 1),
		created: time.Now(),
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	sess.wg.Add(1)
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
		Rows:   sess.rows,
		Cols:   sess.cols,
		Lines:  make([]string, sess.rows),
		Cursor: screen.Position{Row: sess.curRow, Col: sess.curCol},
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

// WaitForText blocks until the given text appears on screen or timeout elapses.
func (m *Manager) WaitForText(id string, text string, timeout time.Duration) error {
	sess, err := m.get(id)
	if err != nil {
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		snap, err := m.Screenshot(id)
		if err != nil {
			return err
		}
		for _, line := range snap.Lines {
			if strings.Contains(line, text) {
				return nil
			}
		}
		select {
		case <-sess.updated:
		case <-timer.C:
			return fmt.Errorf("timeout waiting for %q after %s", text, timeout)
		case <-sess.done:
			return fmt.Errorf("session closed while waiting for %q", text)
		}
	}
}

// WaitForStable blocks until the screen stops changing for stable duration or timeout elapses.
func (m *Manager) WaitForStable(id string, stable time.Duration, timeout time.Duration) error {
	sess, err := m.get(id)
	if err != nil {
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Seed initial state so an already-stable screen returns success immediately.
	initSnap, err := m.Screenshot(id)
	if err != nil {
		return err
	}
	lastSnap := initSnap.Text()
	stableSince := time.Now()

	for {
		if time.Since(stableSince) >= stable {
			return nil
		}
		select {
		case <-sess.updated:
		case <-timer.C:
			return fmt.Errorf("timeout waiting for screen stability after %s", timeout)
		case <-sess.done:
			return fmt.Errorf("session closed while waiting for stability")
		}
		snap, err := m.Screenshot(id)
		if err != nil {
			return err
		}
		current := snap.Text()
		if current != lastSnap {
			lastSnap = current
			stableSince = time.Now()
		}
	}
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

	sess.ptmx.Close()     // unblocks readLoop's Read()
	sess.wg.Wait()        // wait for readLoop to exit
	sess.once.Do(func() { // safe close of done channel
		close(sess.done)
	})
	if sess.Cmd.Process != nil {
		sess.Cmd.Process.Kill()
	}
	if sess.Cmd.Process != nil {
		sess.Cmd.Wait()
	}
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
	defer s.wg.Done()

	buf := make([]byte, 4096)

	for {
		n, err := s.ptmx.Read(buf)
		if err != nil {
			return
		}

		// Copy current buffer state into a local working copy.
		s.mu.RLock()
		localBuf := make([][]rune, s.rows)
		for i, r := range s.buf {
			localBuf[i] = make([]rune, len(r))
			copy(localBuf[i], r)
		}
		rows := s.rows
		cols := s.cols
		row := s.curRow
		col := s.curCol
		s.mu.RUnlock()

		for _, b := range buf[:n] {
			switch b {
			case '\n':
				row++
				col = 0
				if row >= rows {
					// Scroll up
					copy(localBuf, localBuf[1:])
					localBuf[rows-1] = make([]rune, cols)
					for j := range localBuf[rows-1] {
						localBuf[rows-1][j] = ' '
					}
					row = rows - 1
				}
			case '\r':
				col = 0
			default:
				if row < rows && col < cols {
					localBuf[row][col] = rune(b)
					col++
				}
			}
		}

		s.mu.Lock()
		s.buf = localBuf
		s.curRow = row
		s.curCol = col
		s.mu.Unlock()

		// Non-blocking notify
		select {
		case s.updated <- struct{}{}:
		default:
		}
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
