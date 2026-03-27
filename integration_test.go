//go:build integration

package awn_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/tom/awn"
	"github.com/tom/awn/awtreestrategy"
)

func TestIntegration_LazygitDetectsElements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Lazygit needs a git repo to run in.
	if _, err := exec.LookPath("lazygit"); err != nil {
		t.Skip("lazygit not found")
	}

	d := awn.NewDriver()
	defer d.CloseAll()

	s, err := d.SessionWithConfig(awn.Config{
		Command: "lazygit",
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	// Wait for lazygit to render. Use WaitForText since WaitForStable can
	// time out due to lazygit's cursor blinking / clock updates.
	err = s.WaitForText("Status", 10*time.Second)
	if err != nil {
		t.Logf("screen text:\n%s", s.Text())
		t.Fatalf("WaitForText: %v", err)
	}
	// Give it a moment to finish rendering all panels.
	time.Sleep(200 * time.Millisecond)

	// Screenshot to see what's there.
	scr := s.Screen()
	t.Logf("screen (%dx%d):\n%s", scr.Rows, scr.Cols, scr.Text())

	// Run element detection.
	strategy := awtreestrategy.New()
	elements := s.FindAll(strategy)

	t.Logf("detected %d elements:", len(elements))
	for _, e := range elements {
		focused := ""
		if e.Focused {
			focused = " (focused)"
		}
		t.Logf("  %s: %q at %d,%d %dx%d%s",
			e.Type, e.Label, e.Bounds.Row, e.Bounds.Col,
			e.Bounds.Width, e.Bounds.Height, focused)
	}

	// Lazygit should have at least some panels.
	if len(elements) == 0 {
		t.Fatal("expected at least 1 detected element from lazygit UI")
	}

	// Verify we can find elements by type.
	panels := 0
	for _, e := range elements {
		if e.Type == "panel" {
			panels++
		}
	}
	t.Logf("found %d panels", panels)

	// Try FindOne.
	_, err = s.FindOne(strategy, awn.ByType("panel"))
	if err != nil {
		t.Logf("no panel found via FindOne: %v", err)
	}

	// Log serialized output for human inspection.
	for _, e := range elements {
		label := e.Type + ":" + e.Label
		if e.Focused {
			label += "*"
		}
		t.Logf("[%s %d,%d %dx%d]", label, e.Bounds.Row, e.Bounds.Col, e.Bounds.Width, e.Bounds.Height)
	}
}
