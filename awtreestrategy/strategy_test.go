package awtreestrategy

import (
	"testing"

	"github.com/tom/awn"
)

func TestDetect_EmptyScreen_ReturnsNoElements(t *testing.T) {
	s := New()
	screen := makeScreen(5, 10)

	elems := s.Detect(screen)
	if len(elems) != 0 {
		t.Fatalf("expected 0 elements, got %d", len(elems))
	}
}

func TestDetect_PanelOnScreen_ReturnsPanel(t *testing.T) {
	s := New()
	screen := makeScreen(5, 20)

	// Draw a box using box-drawing chars
	setCell(screen, 0, 0, '┌', awn.DefaultColor, awn.DefaultColor, 0)
	for c := 1; c < 9; c++ {
		setCell(screen, 0, c, '─', awn.DefaultColor, awn.DefaultColor, 0)
	}
	setCell(screen, 0, 9, '┐', awn.DefaultColor, awn.DefaultColor, 0)
	for r := 1; r < 4; r++ {
		setCell(screen, r, 0, '│', awn.DefaultColor, awn.DefaultColor, 0)
		setCell(screen, r, 9, '│', awn.DefaultColor, awn.DefaultColor, 0)
	}
	setCell(screen, 4, 0, '└', awn.DefaultColor, awn.DefaultColor, 0)
	for c := 1; c < 9; c++ {
		setCell(screen, 4, c, '─', awn.DefaultColor, awn.DefaultColor, 0)
	}
	setCell(screen, 4, 9, '┘', awn.DefaultColor, awn.DefaultColor, 0)

	elems := s.Detect(screen)
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element (panel)")
	}

	found := false
	for _, e := range elems {
		if e.Type == "panel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a panel element")
	}
}

func TestDetect_ButtonOnScreen_ReturnsButton(t *testing.T) {
	s := New()
	screen := makeScreen(3, 20)

	setText(screen, 1, 5, "[Save]", awn.DefaultColor, awn.DefaultColor, 0)

	elems := s.Detect(screen)

	found := false
	for _, e := range elems {
		if e.Type == "button" && e.Label == "Save" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a button element with label 'Save'")
	}
}

func TestDetect_ReverseButton_IsFocused(t *testing.T) {
	s := New()
	screen := makeScreen(3, 20)

	setText(screen, 1, 5, "[OK]", awn.DefaultColor, awn.DefaultColor, awn.AttrReverse)

	elems := s.Detect(screen)

	found := false
	for _, e := range elems {
		if e.Type == "button" && e.Focused {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a focused button element")
	}
}

// makeScreen creates an awn.Screen filled with spaces.
func makeScreen(rows, cols int) *awn.Screen {
	cells := make([][]awn.Cell, rows)
	for r := range cells {
		cells[r] = make([]awn.Cell, cols)
		for c := range cells[r] {
			cells[r][c] = awn.Cell{Char: ' ', FG: awn.DefaultColor, BG: awn.DefaultColor}
		}
	}
	return &awn.Screen{Rows: rows, Cols: cols, Cells: cells}
}

func setCell(s *awn.Screen, row, col int, ch rune, fg, bg awn.Color, attrs awn.Attr) {
	s.Cells[row][col] = awn.Cell{Char: ch, FG: fg, BG: bg, Attrs: attrs}
}

func setText(s *awn.Screen, row, col int, text string, fg, bg awn.Color, attrs awn.Attr) {
	i := 0
	for _, ch := range text {
		setCell(s, row, col+i, ch, fg, bg, attrs)
		i++
	}
}
