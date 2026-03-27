package awn

import (
	"testing"
)

func TestScreen_Text_ReturnsNewlineJoined(t *testing.T) {
	s := &Screen{
		Rows: 2,
		Cols: 5,
		Cells: [][]Cell{
			{
				{Char: 'h'}, {Char: 'e'}, {Char: 'l'}, {Char: 'l'}, {Char: 'o'},
			},
			{
				{Char: 'w'}, {Char: 'o'}, {Char: 'r'}, {Char: 'l'}, {Char: 'd'},
			},
		},
	}
	got := s.Text()
	want := "hello\nworld"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestScreen_Lines_ReturnsPerLineContent(t *testing.T) {
	s := &Screen{
		Rows: 2,
		Cols: 5,
		Cells: [][]Cell{
			{
				{Char: 'h'}, {Char: 'e'}, {Char: 'l'}, {Char: 'l'}, {Char: 'o'},
			},
			{
				{Char: 'w'}, {Char: 'o'}, {Char: 'r'}, {Char: 'l'}, {Char: 'd'},
			},
		},
	}
	got := s.Lines()
	if len(got) != 2 {
		t.Fatalf("Lines() len = %d, want 2", len(got))
	}
	if got[0] != "hello" {
		t.Errorf("Lines()[0] = %q, want %q", got[0], "hello")
	}
	if got[1] != "world" {
		t.Errorf("Lines()[1] = %q, want %q", got[1], "world")
	}
}
