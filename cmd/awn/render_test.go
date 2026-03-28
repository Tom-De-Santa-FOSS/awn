package main

import (
	"strings"
	"testing"
)

func TestRenderLines_produces_screen_content(t *testing.T) {
	lines := []string{"hello", "world"}
	out := renderLines(lines)
	if !strings.Contains(out, "hello") {
		t.Errorf("output missing 'hello': %q", out)
	}
	if !strings.Contains(out, "world") {
		t.Errorf("output missing 'world': %q", out)
	}
}

func TestRenderStatusBar_contains_session_id_and_state(t *testing.T) {
	bar := renderStatusBar("sess-abc", "idle")
	if !strings.Contains(bar, "sess-abc") {
		t.Errorf("status bar missing session ID: %q", bar)
	}
	if !strings.Contains(bar, "idle") {
		t.Errorf("status bar missing state: %q", bar)
	}
}

func TestRenderLines_starts_with_clear_screen(t *testing.T) {
	lines := []string{"hi"}
	out := renderLines(lines)
	if !strings.HasPrefix(out, "\033[2J\033[H") {
		t.Errorf("expected clear screen prefix, got %q", out[:min(len(out), 20)])
	}
}
