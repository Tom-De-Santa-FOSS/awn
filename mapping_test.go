package awn

import (
	"testing"

	vt10x "github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
)

func TestMapColor_vt10x_defaults_become_DefaultColor(t *testing.T) {
	assert.Equal(t, DefaultColor, mapColor(vt10x.DefaultFG))
	assert.Equal(t, DefaultColor, mapColor(vt10x.DefaultBG))
}

func TestMapColor_palette_index_preserved(t *testing.T) {
	assert.Equal(t, Color(1), mapColor(vt10x.Color(1)))
	assert.Equal(t, Color(255), mapColor(vt10x.Color(255)))
}

func TestMapAttrs_reverse(t *testing.T) {
	assert.Equal(t, AttrReverse, mapAttrs(vt10xModeReverse))
}

func TestMapAttrs_multiple(t *testing.T) {
	mode := int16(vt10xModeBold | vt10xModeUnderline | vt10xModeItalic)
	a := mapAttrs(mode)
	assert.True(t, a&AttrBold != 0)
	assert.True(t, a&AttrUnderline != 0)
	assert.True(t, a&AttrItalic != 0)
	assert.True(t, a&AttrReverse == 0)
}

// Regression: pin vt10x mode bit positions. If vt10x changes its
// unexported attr iota order, this test catches it by round-tripping
// through a real vt10x terminal with ANSI escape sequences.
func TestMapAttrs_matches_vt10x_terminal(t *testing.T) {
	term := vt10x.New(vt10x.WithSize(2, 10))

	// SGR 1 = bold, 4 = underline, 7 = reverse
	term.Write([]byte("\033[1;4;7mHi"))
	g := term.Cell(0, 0)
	a := mapAttrs(g.Mode)
	assert.True(t, a&AttrBold != 0, "bold not detected from real vt10x terminal")
	assert.True(t, a&AttrUnderline != 0, "underline not detected from real vt10x terminal")
	assert.True(t, a&AttrReverse != 0, "reverse not detected from real vt10x terminal")
}
