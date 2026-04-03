package awn

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"
)

type castEvent struct {
	Time float64 `json:"time"`
	Type string  `json:"type"`
	Data string  `json:"data"`
}

type persistedSession struct {
	ID        string      `json:"id"`
	Config    Config      `json:"config"`
	Screen    *Screen     `json:"screen"`
	History   []string    `json:"history,omitempty"`
	Events    []castEvent `json:"events,omitempty"`
	StartedAt time.Time   `json:"started_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type historyBuffer struct {
	maxLines int
	lines    []string
	pending  string
}

func newHistoryBuffer(maxLines int) *historyBuffer {
	return &historyBuffer{maxLines: maxLines}
}

func (h *historyBuffer) append(data string) {
	if h == nil || data == "" {
		return
	}
	data = strings.ReplaceAll(data, "\r\n", "\n")
	parts := strings.Split(h.pending+data, "\n")
	h.pending = parts[len(parts)-1]
	h.lines = append(h.lines, parts[:len(parts)-1]...)
	h.trim()
}

func (h *historyBuffer) snapshot() []string {
	if h == nil {
		return nil
	}
	lines := append([]string(nil), h.lines...)
	if h.pending != "" {
		lines = append(lines, h.pending)
	}
	return lines
}

func (h *historyBuffer) load(lines []string) {
	if h == nil {
		return
	}
	h.lines = append([]string(nil), lines...)
	h.pending = ""
	h.trim()
}

func (h *historyBuffer) trim() {
	if h.maxLines > 0 && len(h.lines) > h.maxLines {
		h.lines = slices.Clone(h.lines[len(h.lines)-h.maxLines:])
	}
	if h.maxLines > 0 && len(h.lines) == h.maxLines && h.pending != "" {
		h.lines = slices.Clone(h.lines[1:])
	}
	if h.maxLines == 0 {
		h.lines = nil
		h.pending = ""
	}
}

func cloneScreen(src *Screen) *Screen {
	if src == nil {
		return nil
	}
	clone := &Screen{
		Rows:   src.Rows,
		Cols:   src.Cols,
		Cursor: src.Cursor,
		Cells:  make([][]Cell, len(src.Cells)),
	}
	for i := range src.Cells {
		clone.Cells[i] = append([]Cell(nil), src.Cells[i]...)
	}
	return clone
}

func (s *Session) appendOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	now := time.Now()
	if s.history != nil {
		s.history.append(string(data))
	}
	s.events = append(s.events, castEvent{
		Time: now.Sub(s.startedAt).Seconds(),
		Type: "o",
		Data: string(data),
	})
	s.updatedAt = now
	if s.snapshot != nil {
		s.snapshot = cloneScreen(s.snapshot)
	}
}

func (s *Session) Scrollback(limit int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lines := s.history.snapshot()
	if limit > 0 && len(lines) > limit {
		return append([]string(nil), lines[len(lines)-limit:]...)
	}
	return lines
}

func (s *Session) RecordAsciicast(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	header := struct {
		Version   int               `json:"version"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		Timestamp int64             `json:"timestamp"`
		Env       map[string]string `json:"env,omitempty"`
	}{
		Version:   2,
		Width:     s.cfg.Cols,
		Height:    s.cfg.Rows,
		Timestamp: s.startedAt.Unix(),
		Env:       map[string]string{"TERM": "xterm-256color"},
	}
	if header.Width == 0 || header.Height == 0 {
		scr := s.screenLocked()
		header.Width = scr.Cols
		header.Height = scr.Rows
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck
	enc := json.NewEncoder(file)
	if err := enc.Encode(header); err != nil {
		return err
	}
	for _, event := range s.events {
		if err := enc.Encode([]any{event.Time, event.Type, event.Data}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) persistentState() *persistedSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &persistedSession{
		ID:        s.ID,
		Config:    s.cfg,
		Screen:    s.screenLocked(),
		History:   s.history.snapshot(),
		Events:    append([]castEvent(nil), s.events...),
		StartedAt: s.startedAt,
		UpdatedAt: s.updatedAt,
	}
}

func newRestoredSession(state *persistedSession) *Session {
	if state == nil || state.Screen == nil {
		return nil
	}
	history := newHistoryBuffer(state.Config.Scrollback)
	history.load(state.History)
	return &Session{
		ID:        state.ID,
		done:      make(chan struct{}),
		updated:   make(chan struct{}, 1),
		cfg:       state.Config,
		history:   history,
		events:    append([]castEvent(nil), state.Events...),
		snapshot:  cloneScreen(state.Screen),
		restored:  true,
		startedAt: state.StartedAt,
		updatedAt: state.UpdatedAt,
	}
}
