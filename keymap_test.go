package awn

import "testing"

func TestResolveKey_Named(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Enter", "\r"},
		{"Tab", "\t"},
		{"Backspace", "\x7f"},
		{"Escape", "\x1b"},
		{"Space", " "},
		{"Delete", "\x1b[3~"},
		{"ArrowUp", "\x1b[A"},
		{"Up", "\x1b[A"},
		{"ArrowDown", "\x1b[B"},
		{"Down", "\x1b[B"},
		{"ArrowRight", "\x1b[C"},
		{"Right", "\x1b[C"},
		{"ArrowLeft", "\x1b[D"},
		{"Left", "\x1b[D"},
		{"Home", "\x1b[H"},
		{"End", "\x1b[F"},
		{"PageUp", "\x1b[5~"},
		{"PageDown", "\x1b[6~"},
		{"F1", "\x1bOP"},
		{"F12", "\x1b[24~"},
		{"Ctrl+C", "\x03"},
		{"Ctrl+D", "\x04"},
		{"Ctrl+Z", "\x1a"},
		{"Ctrl+L", "\x0c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveKey(tt.name)
			if !ok {
				t.Fatalf("ResolveKey(%q) returned not-ok", tt.name)
			}
			if got != tt.want {
				t.Fatalf("ResolveKey(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestResolveKey_SingleChar(t *testing.T) {
	got, ok := ResolveKey("a")
	if !ok {
		t.Fatal("single char should resolve")
	}
	if got != "a" {
		t.Fatalf("got %q, want %q", got, "a")
	}
}

func TestResolveKey_Unknown(t *testing.T) {
	_, ok := ResolveKey("NonExistentKey")
	if ok {
		t.Fatal("unknown multi-char key should return not-ok")
	}
}
