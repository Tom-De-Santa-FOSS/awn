package main

import (
	"path/filepath"
	"testing"
)

func TestResolveStateDir_PrefersExplicitEnvValue(t *testing.T) {
	got := resolveStateDir("/tmp/custom", func() (string, error) {
		t.Fatal("cache dir lookup should not run when env is set")
		return "", nil
	})
	if got != "/tmp/custom" {
		t.Fatalf("resolveStateDir = %q, want %q", got, "/tmp/custom")
	}
}

func TestResolveStateDir_UsesUserCacheDirWhenEnvUnset(t *testing.T) {
	got := resolveStateDir("", func() (string, error) {
		return "/tmp/cache", nil
	})
	want := filepath.Join("/tmp/cache", "awn", "sessions")
	if got != want {
		t.Fatalf("resolveStateDir = %q, want %q", got, want)
	}
}
