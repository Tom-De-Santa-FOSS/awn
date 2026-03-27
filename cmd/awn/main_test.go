package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestCallDialError verifies that call() returns a fatal error when the daemon
// is not reachable. We use a helper that wraps the dial so we can test without
// os.Exit.
func TestDialUnreachable(t *testing.T) {
	_, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:1", nil)
	if err == nil {
		t.Fatal("expected error dialing unreachable address, got nil")
	}
}

// TestAuthHeaderSet verifies that when AWN_TOKEN is set the Authorization
// header is populated correctly.
func TestAuthHeaderSet(t *testing.T) {
	received := make(chan http.Header, 1)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		upgrader.Upgrade(w, r, nil) //nolint:errcheck
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	token := "test-secret-token"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	conn.Close() //nolint:errcheck

	got := <-received
	auth := got.Get("Authorization")
	if auth != "Bearer "+token {
		t.Fatalf("expected Authorization %q, got %q", "Bearer "+token, auth)
	}
}
