package awn

type StructuredStrategy interface {
	Strategy
	DetectStructured(screen *Screen) *DetectResult
}

type DetectResult struct {
	Elements []DetectElement  `json:"elements"`
	Tree     []DetectTreeNode `json:"tree,omitempty"`
	Viewport Rect             `json:"viewport"`
	Scrolled bool             `json:"scrolled"`
}

type DetectElement struct {
	ID            int    `json:"id"`
	Type          string `json:"type"`
	Label         string `json:"label"`
	Description   string `json:"description,omitempty"`
	Bounds        Rect   `json:"bounds"`
	Focused       bool   `json:"focused"`
	Enabled       bool   `json:"enabled"`
	Checked       bool   `json:"checked"`
	Selected      bool   `json:"selected"`
	Visible       bool   `json:"visible"`
	Clipped       bool   `json:"clipped"`
	Role          string `json:"role,omitempty"`
	Shortcut      string `json:"shortcut,omitempty"`
	Ref           string `json:"ref,omitempty"`
	VisibleBounds *Rect  `json:"visible_bounds,omitempty"`
	Children      []int  `json:"children,omitempty"`
}

type DetectTreeNode struct {
	ID            int              `json:"id"`
	Type          string           `json:"type"`
	Label         string           `json:"label"`
	Description   string           `json:"description,omitempty"`
	Bounds        Rect             `json:"bounds"`
	Focused       bool             `json:"focused"`
	Enabled       bool             `json:"enabled"`
	Checked       bool             `json:"checked"`
	Selected      bool             `json:"selected"`
	Visible       bool             `json:"visible"`
	Clipped       bool             `json:"clipped"`
	Role          string           `json:"role,omitempty"`
	Shortcut      string           `json:"shortcut,omitempty"`
	Ref           string           `json:"ref,omitempty"`
	VisibleBounds *Rect            `json:"visible_bounds,omitempty"`
	Children      []DetectTreeNode `json:"children,omitempty"`
}
