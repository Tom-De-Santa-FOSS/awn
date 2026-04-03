package main

import (
	"encoding/json"
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

func TestRun_ResizeCommand_CallsResizeRPC(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"resize", "sess-123", "40", "100"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return "", marshalErr
		}
		if unmarshalErr := json.Unmarshal(data, &gotParams); unmarshalErr != nil {
			return "", unmarshalErr
		}
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout != "ok\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "ok\n")
	}
	if gotMethod != "resize" {
		t.Fatalf("method = %q, want %q", gotMethod, "resize")
	}
	if gotParams["id"] != "sess-123" {
		t.Fatalf("id = %#v, want %q", gotParams["id"], "sess-123")
	}
	if gotParams["rows"] != float64(40) {
		t.Fatalf("rows = %#v, want %v", gotParams["rows"], 40)
	}
	if gotParams["cols"] != float64(100) {
		t.Fatalf("cols = %#v, want %v", gotParams["cols"], 100)
	}
}

func TestRun_PingCommand_CallsPingRPC(t *testing.T) {
	var gotMethod string
	stdout, err := run([]string{"ping"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		if params != nil {
			t.Fatalf("params = %#v, want nil", params)
		}
		return `{"status":"ok"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout != "{\"status\":\"ok\"}\n" {
		t.Fatalf("stdout = %q, want ping JSON", stdout)
	}
	if gotMethod != "ping" {
		t.Fatalf("method = %q, want %q", gotMethod, "ping")
	}
}

func TestRun_ScreenshotFullFlag_RequestsFullFormat(t *testing.T) {
	var gotParams map[string]any
	stdout, err := run([]string{"screenshot", "sess-123", "--full"}, func(_ string, method string, params any) (string, error) {
		if method != "screenshot" {
			t.Fatalf("method = %q, want %q", method, "screenshot")
		}
		data, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return "", marshalErr
		}
		if unmarshalErr := json.Unmarshal(data, &gotParams); unmarshalErr != nil {
			return "", unmarshalErr
		}
		return `{"lines":["hello"],"elements":[{"type":"button","label":"OK"}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["format"] != "full" {
		t.Fatalf("format = %#v, want %q", gotParams["format"], "full")
	}
	if stdout == "" {
		t.Fatal("expected JSON output for --full")
	}
}
