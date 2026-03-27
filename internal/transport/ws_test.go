package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// mockDispatcher is a test double for the Dispatcher interface.
type mockDispatcher struct {
	result any
	err    error
}

func (m *mockDispatcher) Dispatch(method string, params json.RawMessage) (any, error) {
	return m.result, m.err
}

// dialWS connects to an httptest.Server via WebSocket, optionally with an auth header.
func dialWS(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}
	return websocket.DefaultDialer.Dial(url, hdr)
}

// dialWSWithOrigin connects with an explicit Origin header.
func dialWSWithOrigin(t *testing.T, srv *httptest.Server, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial(url, hdr)
}

func newTestServer(t *testing.T, d Dispatcher, token string) *httptest.Server {
	t.Helper()
	s := NewServer(d, "", token)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWS)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	return httptest.NewServer(mux)
}

// --- Tests ---

func TestHealthEndpoint(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestWebSocketUpgradeNoToken(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()
}

func TestWebSocketUpgrade401WhenTokenSetAndNoAuthHeader(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "secret")
	defer srv.Close()

	_, resp, err := dialWS(t, srv, "")
	if err == nil {
		t.Fatal("expected dial to fail, got nil error")
	}
	if resp == nil {
		t.Fatal("expected HTTP response, got nil")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocketUpgrade401WhenTokenSetAndWrongToken(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "secret")
	defer srv.Close()

	// Use custom dialer to send wrong Bearer token.
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer wrongtoken")
	_, resp, err := websocket.DefaultDialer.Dial(url, hdr)
	if err == nil {
		t.Fatal("expected dial to fail with wrong token")
	}
	if resp == nil {
		t.Fatal("expected HTTP response, got nil")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocketUpgradeSucceedsWithCorrectToken(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "secret")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "secret")
	if err != nil {
		t.Fatalf("dial with correct token: %v", err)
	}
	conn.Close()
}

func TestWebSocketUpgradeRejectedWithOriginHeader(t *testing.T) {
	d := &mockDispatcher{}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	_, resp, err := dialWSWithOrigin(t, srv, "http://evil.example.com")
	if err == nil {
		t.Fatal("expected dial to fail when Origin is set")
	}
	if resp == nil {
		t.Fatal("expected HTTP response, got nil")
	}
	// gorilla returns 403 when CheckOrigin returns false
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestErrorResponseUsesGenericMessage(t *testing.T) {
	d := &mockDispatcher{err: errors.New("super secret internal details")}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "anything",
		ID:      1,
	}
	data, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error in response, got nil")
	}
	if resp.Error.Message == "super secret internal details" {
		t.Errorf("error message must not expose internal details, got %q", resp.Error.Message)
	}
	if resp.Error.Message != "internal error" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "internal error")
	}
}

func TestValidJSONRPCRequestDispatchesCorrectly(t *testing.T) {
	expected := map[string]string{"id": "abc-123"}
	d := &mockDispatcher{result: expected}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "create",
		Params:  json.RawMessage(`{"command":"bash"}`),
		ID:      42,
	}
	data, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", resp.JSONRPC, "2.0")
	}
	// ID round-trips as float64 through JSON
	if resp.ID == nil {
		t.Error("expected non-nil ID")
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}
