package main

import (
	"sort"
	"testing"
)

func TestNewServer_registers_all_eight_tools(t *testing.T) {
	s := newServer(nil)
	tools := s.ListTools()

	want := []string{
		"awn_close",
		"awn_create",
		"awn_detect",
		"awn_input",
		"awn_list",
		"awn_screenshot",
		"awn_wait_for_stable",
		"awn_wait_for_text",
	}

	if len(tools) != len(want) {
		t.Fatalf("got %d tools, want %d", len(tools), len(want))
	}

	var got []string
	for name := range tools {
		got = append(got, name)
	}
	sort.Strings(got)

	for i, name := range want {
		if got[i] != name {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], name)
		}
	}
}
