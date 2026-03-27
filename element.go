package awn

// Strategy detects UI elements from a terminal screen.
type Strategy interface {
	Detect(screen *Screen) []Element
}

// Element is a detected UI component.
type Element struct {
	Type    string
	Label   string
	Bounds  Rect
	Focused bool
}

// Rect describes the position and size of an element on screen.
type Rect struct {
	Row    int
	Col    int
	Width  int
	Height int
}

// MatchFunc filters elements.
type MatchFunc func(Element) bool

// ByLabel returns a MatchFunc that matches elements with the given label.
func ByLabel(label string) MatchFunc {
	return func(e Element) bool {
		return e.Label == label
	}
}

// ByType returns a MatchFunc that matches elements with the given type.
func ByType(typ string) MatchFunc {
	return func(e Element) bool {
		return e.Type == typ
	}
}
