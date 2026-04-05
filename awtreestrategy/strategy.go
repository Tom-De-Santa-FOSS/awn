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
	m := s.DetectStructured(screen)

	elements := make([]awn.Element, len(m.Elements))
	for i, e := range m.Elements {
		elements[i] = awn.Element{
			Type:    e.Type,
			Label:   e.Label,
			Focused: e.Focused,
			Bounds:  e.Bounds,
		}
	}
	return elements
}

func (s *Strategy) DetectStructured(screen *awn.Screen) *awn.DetectResult {
	g := toGrid(screen)
	m := awtree.Detect(g)

	result := &awn.DetectResult{
		Elements: make([]awn.DetectElement, len(m.Elements)),
		Viewport: awn.Rect{Row: m.Viewport.Row, Col: m.Viewport.Col, Width: m.Viewport.Width, Height: m.Viewport.Height},
		Scrolled: m.Scrolled,
	}
	for i, e := range m.Elements {
		result.Elements[i] = awn.DetectElement{
			ID:          e.ID,
			Type:        e.Type.String(),
			Label:       e.Label,
			Description: e.Description,
			Bounds:      awn.Rect{Row: e.Bounds.Row, Col: e.Bounds.Col, Width: e.Bounds.Width, Height: e.Bounds.Height},
			Focused:     e.Focused,
			Enabled:     e.Enabled,
			Checked:     e.Checked,
			Selected:    e.Selected,
			Visible:     e.Visible,
			Clipped:     e.Clipped,
			Role:        e.Role,
			Shortcut:    e.Shortcut,
			Ref:         e.Ref,
			Children:    append([]int(nil), e.Children...),
		}
		if e.VisibleBounds != nil {
			result.Elements[i].VisibleBounds = &awn.Rect{Row: e.VisibleBounds.Row, Col: e.VisibleBounds.Col, Width: e.VisibleBounds.Width, Height: e.VisibleBounds.Height}
		}
	}

	byID := make(map[int]awn.DetectElement, len(result.Elements))
	childIDs := make(map[int]bool, len(result.Elements))
	for _, el := range result.Elements {
		byID[el.ID] = el
		for _, childID := range el.Children {
			childIDs[childID] = true
		}
	}
	for _, el := range result.Elements {
		if childIDs[el.ID] {
			continue
		}
		result.Tree = append(result.Tree, buildTreeNode(el, byID))
	}

	return result
}

func buildTreeNode(el awn.DetectElement, byID map[int]awn.DetectElement) awn.DetectTreeNode {
	node := awn.DetectTreeNode{
		ID:            el.ID,
		Type:          el.Type,
		Label:         el.Label,
		Description:   el.Description,
		Bounds:        el.Bounds,
		Focused:       el.Focused,
		Enabled:       el.Enabled,
		Checked:       el.Checked,
		Selected:      el.Selected,
		Visible:       el.Visible,
		Clipped:       el.Clipped,
		Role:          el.Role,
		Shortcut:      el.Shortcut,
		Ref:           el.Ref,
		VisibleBounds: el.VisibleBounds,
	}
	for _, childID := range el.Children {
		child, ok := byID[childID]
		if !ok {
			continue
		}
		node.Children = append(node.Children, buildTreeNode(child, byID))
	}
	return node
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
