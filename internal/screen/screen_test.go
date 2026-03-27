package screen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestText_EmptySnapshot(t *testing.T) {
	s := Snapshot{}
	got := s.Text()
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestText_SingleLine(t *testing.T) {
	s := Snapshot{Lines: []string{"hello"}}
	got := s.Text()
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestText_MultipleLines(t *testing.T) {
	s := Snapshot{Lines: []string{"foo", "bar", "baz"}}
	got := s.Text()
	want := "foo\nbar\nbaz"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSnapshot_JSON_NoGrid(t *testing.T) {
	s := Snapshot{
		Rows:  2,
		Cols:  80,
		Lines: []string{"line1", "line2"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(data), "grid") {
		t.Fatalf("expected no 'grid' key in JSON, got: %s", string(data))
	}
}
