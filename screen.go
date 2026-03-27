package awn

import "strings"

// Color represents a terminal color.
type Color int32

// DefaultColor represents the terminal's default color.
const DefaultColor Color = -1

// Attr represents text display attributes.
type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrConceal
	AttrStrikethrough
)

// Cell holds a single terminal cell's content and styling.
type Cell struct {
	Char  rune
	FG    Color
	BG    Color
	Attrs Attr
}

// Position represents a row/column coordinate.
type Position struct {
	Row int
	Col int
}

// Screen is a snapshot of the terminal display.
type Screen struct {
	Rows   int
	Cols   int
	Cells  [][]Cell
	Cursor Position
}

// Lines returns the text content of each row, trimming trailing spaces.
func (s *Screen) Lines() []string {
	lines := make([]string, s.Rows)
	for row := 0; row < s.Rows; row++ {
		line := make([]rune, s.Cols)
		for col := 0; col < s.Cols; col++ {
			line[col] = s.Cells[row][col].Char
		}
		lines[row] = strings.TrimRight(string(line), " \x00")
	}
	return lines
}

// Text returns the plain text content of all rows joined by newlines.
func (s *Screen) Text() string {
	return strings.Join(s.Lines(), "\n")
}
