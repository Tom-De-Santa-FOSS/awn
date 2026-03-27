package awtreestrategy

import (
	"github.com/Tom-De-Santa-FOSS/awtree"
	"github.com/tom/awn"
)

// Strategy adapts awtree element detection as an awn.Strategy.
type Strategy struct{}

// New creates an awtree-based detection strategy.
func New() *Strategy {
	return &Strategy{}
}

// Detect converts the awn Screen to an awtree Grid, runs detection, and
// converts the results back to awn Elements.
func (s *Strategy) Detect(screen *awn.Screen) []awn.Element {
	g := toGrid(screen)
	m := awtree.Detect(g)

	elements := make([]awn.Element, len(m.Elements))
	for i, e := range m.Elements {
		elements[i] = awn.Element{
			Type:    e.Type.String(),
			Label:   e.Label,
			Focused: e.Focused,
			Bounds: awn.Rect{
				Row:    e.Bounds.Row,
				Col:    e.Bounds.Col,
				Width:  e.Bounds.Width,
				Height: e.Bounds.Height,
			},
		}
	}
	return elements
}

// toGrid converts an awn.Screen to an awtree.Grid.
// The Cell/Color/Attr types share identical field names and bit layouts,
// so this is a straightforward field copy.
func toGrid(screen *awn.Screen) *awtree.Grid {
	g := awtree.NewGrid(screen.Rows, screen.Cols)
	for row := 0; row < screen.Rows; row++ {
		for col := 0; col < screen.Cols; col++ {
			c := screen.Cells[row][col]
			g.Cells[row][col] = awtree.Cell{
				Char:  c.Char,
				FG:    awtree.Color(c.FG),
				BG:    awtree.Color(c.BG),
				Attrs: awtree.Attr(c.Attrs),
			}
		}
	}
	return g
}
