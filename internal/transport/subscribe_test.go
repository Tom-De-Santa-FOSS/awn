package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockSubscriber implements both Dispatcher and Subscriber for testing.
type mockSubscriber struct {
	mockDispatcher
	subscribeID  string
	subscribeFn  func(notify func(json.RawMessage))
	unsubscribed bool
}

func (m *mockSubscriber) Subscribe(sessionID string, notify func(json.RawMessage)) (subID string, err error) {
	if m.subscribeFn != nil {
		m.subscribeFn(notify)
	}
	return m.subscribeID, nil
}

func (m *mockSubscriber) Unsubscribe(sessionID, subID string) {
	m.unsubscribed = true
}

func TestSubscribe_returns_subscribed_response(t *testing.T) {
	d := &mockSubscriber{subscribeID: "sub-1"}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "subscribe",
		Params:  json.RawMessage(`{"id":"sess-1"}`),
		ID:      1,
	}
	data, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["subscribed"] != true {
		t.Errorf("expected subscribed=true, got %v", result["subscribed"])
	}
}

func TestSubscribe_sends_notifications(t *testing.T) {
	notifyCh := make(chan func(json.RawMessage), 1)
	d := &mockSubscriber{
		subscribeID: "sub-1",
		subscribeFn: func(notify func(json.RawMessage)) {
			notifyCh <- notify
		},
	}
	srv := newTestServer(t, d, "")
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "subscribe",
		Params:  json.RawMessage(`{"id":"sess-1"}`),
		ID:      1,
	}
	data, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the subscribe response first.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read subscribe response: %v", err)
	}

	// Get the notify function and send a notification.
	select {
	case notify := <-notifyCh:
		screenData, _ := json.Marshal(map[string]any{"rows": 24, "cols": 80})
		notify(screenData)
	case <-time.After(time.Second):
		t.Fatal("no notify func received")
	}

	// Read the notification.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read notification: %v", err)
	}

	// Notification is a JSON-RPC notification (no id).
	var notif map[string]any
	if err := json.Unmarshal(msg, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif["method"] != "screen_update" {
		t.Errorf("notification method = %v, want screen_update", notif["method"])
	}
	if _, hasID := notif["id"]; hasID {
		t.Error("notification should not have an id field")
	}
}
