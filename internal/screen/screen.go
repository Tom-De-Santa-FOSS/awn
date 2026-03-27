package screen

import "strings"

// Snapshot is a point-in-time capture of the terminal screen.
type Snapshot struct {
	Rows   int      `json:"rows"`
	Cols   int      `json:"cols"`
	Lines  []string `json:"lines"`
	Cursor Position `json:"cursor"`
}

// Position represents cursor location.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Text returns the screen content as a single string with newlines.
func (s *Snapshot) Text() string {
	return strings.Join(s.Lines, "\n")
}
