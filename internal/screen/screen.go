package screen

// Cell represents a single character cell in the terminal grid.
type Cell struct {
	Char  string `json:"char"`
	FG    string `json:"fg,omitempty"`
	BG    string `json:"bg,omitempty"`
	Bold  bool   `json:"bold,omitempty"`
}

// Snapshot is a point-in-time capture of the terminal screen.
type Snapshot struct {
	Rows    int      `json:"rows"`
	Cols    int      `json:"cols"`
	Lines   []string `json:"lines"`
	Grid    [][]Cell `json:"grid,omitempty"`
	Cursor  Position `json:"cursor"`
}

// Position represents cursor location.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Text returns the screen content as a single string with newlines.
func (s *Snapshot) Text() string {
	out := ""
	for i, line := range s.Lines {
		out += line
		if i < len(s.Lines)-1 {
			out += "\n"
		}
	}
	return out
}
