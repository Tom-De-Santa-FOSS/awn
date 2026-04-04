package awn

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

const (
	DefaultRows = 24
	DefaultCols = 80
)

// Driver manages terminal sessions.
type Driver struct {
	sessions       map[string]*Session
	mu             sync.RWMutex
	pty            PTYStarter
	persistenceDir string
}

// NewDriver creates a new Driver.
func NewDriver(opts ...DriverOption) *Driver {
	d := &Driver{
		sessions: make(map[string]*Session),
		pty:      realPTY{},
	}
	for _, opt := range opts {
		opt(d)
	}
	d.loadPersistedSessions()
	return d
}

// Session creates a new terminal session running the given command.
func (d *Driver) Session(command string, args ...string) (*Session, error) {
	return d.SessionWithConfig(Config{
		Command: command,
		Args:    args,
		Rows:    DefaultRows,
		Cols:    DefaultCols,
	})
}

// SessionWithConfig creates a new terminal session with explicit configuration.
func (d *Driver) SessionWithConfig(cfg Config) (*Session, error) {
	if cfg.Rows == 0 {
		cfg.Rows = DefaultRows
	}
	if cfg.Cols == 0 {
		cfg.Cols = DefaultCols
	}
	if cfg.Scrollback == 0 {
		cfg.Scrollback = 1000
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		fmt.Sprintf("COLUMNS=%d", cfg.Cols),
		fmt.Sprintf("LINES=%d", cfg.Rows),
	)

	ptmx, err := d.pty.Start(cmd, &pty.Winsize{
		Rows: uint16(cfg.Rows),
		Cols: uint16(cfg.Cols),
	})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	id := shortID()
	sess := &Session{
		ID:        id,
		cmd:       cmd,
		ptmx:      ptmx,
		term:      vt10x.New(vt10x.WithSize(cfg.Cols, cfg.Rows)),
		done:      make(chan struct{}),
		updated:   make(chan struct{}, 1),
		cfg:       cfg,
		history:   newHistoryBuffer(cfg.Scrollback),
		startedAt: time.Now(),
		updatedAt: time.Now(),
	}
	if d.persistenceDir != "" {
		sess.persist = func() { d.persistSession(sess) }
	}

	d.mu.Lock()
	d.sessions[id] = sess
	d.mu.Unlock()

	sess.wg.Add(1)
	go sess.readLoop()
	d.persistSession(sess)

	return sess, nil
}

// List returns all active session IDs.
func (d *Driver) List() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ids := make([]string, 0, len(d.sessions))
	for id := range d.sessions {
		ids = append(ids, id)
	}
	return ids
}

// shortID generates a random 8-character hex string.
func shortID() string {
	var b [4]byte
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return hex.EncodeToString(b[:])
}

// Get returns a session by ID or unique prefix, or nil if not found.
func (d *Driver) Get(id string) *Session {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.findSession(id)
}

// findSession looks up by exact match first, then prefix match.
// Must be called with d.mu held.
func (d *Driver) findSession(id string) *Session {
	if s, ok := d.sessions[id]; ok {
		return s
	}
	var match *Session
	for sid, s := range d.sessions {
		if strings.HasPrefix(sid, id) {
			if match != nil {
				return nil // ambiguous prefix
			}
			match = s
		}
	}
	return match
}

// resolveID returns the full session ID for an exact or prefix match.
// Must be called with d.mu held.
func (d *Driver) resolveID(id string) (string, bool) {
	if _, ok := d.sessions[id]; ok {
		return id, true
	}
	var found string
	for sid := range d.sessions {
		if strings.HasPrefix(sid, id) {
			if found != "" {
				return "", false // ambiguous
			}
			found = sid
		}
	}
	return found, found != ""
}

// Close terminates a session by ID or unique prefix.
func (d *Driver) Close(id string) error {
	d.mu.Lock()
	fullID, ok := d.resolveID(id)
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("session %q not found", id)
	}
	sess := d.sessions[fullID]
	delete(d.sessions, fullID)
	d.mu.Unlock()
	sess.stopPersisting()

	err := sess.Close()
	d.deletePersistedSession(fullID)
	return err
}

// CloseAll terminates all active sessions.
func (d *Driver) CloseAll() {
	d.mu.Lock()
	sessions := make([]*Session, 0, len(d.sessions))
	for _, sess := range d.sessions {
		sessions = append(sessions, sess)
	}
	d.sessions = make(map[string]*Session)
	d.mu.Unlock()

	for _, sess := range sessions {
		_ = sess.Close()
	}
}

// Config holds session creation parameters.
type Config struct {
	Command    string   `json:"command"`
	Args       []string `json:"args,omitempty"`
	Rows       int      `json:"rows,omitempty"`
	Cols       int      `json:"cols,omitempty"`
	Scrollback int      `json:"scrollback,omitempty"`
}

func (d *Driver) persistSession(sess *Session) {
	if d.persistenceDir == "" || sess == nil {
		return
	}
	state := sess.persistentState()
	if state == nil {
		return
	}
	if err := os.MkdirAll(d.persistenceDir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(d.persistenceDir, sess.ID+".json"), data, 0o644)
}

func (d *Driver) deletePersistedSession(id string) {
	if d.persistenceDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(d.persistenceDir, id+".json"))
}

func (d *Driver) loadPersistedSessions() {
	if d.persistenceDir == "" {
		return
	}
	entries, err := os.ReadDir(d.persistenceDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(d.persistenceDir, entry.Name()))
		if err != nil {
			continue
		}
		var state persistedSession
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}
		sess := newRestoredSession(&state)
		if sess == nil {
			continue
		}
		if d.persistenceDir != "" {
			sess.persist = func() { d.persistSession(sess) }
		}
		d.sessions[sess.ID] = sess
	}
}
