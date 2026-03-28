package transport

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/tom/awn"
	"github.com/tom/awn/internal/rpc"
)

// pipePTY is a fake PTYStarter that returns a pipe instead of a real PTY.
type pipePTY struct {
	W *os.File
}

func (p *pipePTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.W = w
	return r, nil
}

type stubStrategy struct{}

func (stubStrategy) Detect(screen *awn.Screen) []awn.Element { return nil }

func TestIntegration_subscribe_receives_screen_updates(t *testing.T) {
	// 1. Create a real awn.Driver with a fake PTY.
	p := &pipePTY{}
	driver := awn.NewDriver(awn.WithPTY(p))

	// 2. Create an rpc.Handler with a stubStrategy.
	handler := rpc.NewHandler(driver, stubStrategy{})

	// 3. Start a test WebSocket server.
	srv := newTestServer(t, handler, "")
	defer srv.Close()

	// 4. Connect a WebSocket client.
	conn, _, err := dialWS(t, srv, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	// 5. Create a session via JSON-RPC "create" method.
	createReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "create",
		Params:  json.RawMessage(`{"command":"true"}`),
		ID:      1,
	}
	data, _ := json.Marshal(createReq)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write create: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read create response: %v", err)
	}

	var createResp JSONRPCResponse
	if err := json.Unmarshal(msg, &createResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if createResp.Error != nil {
		t.Fatalf("create error: %v", createResp.Error)
	}

	resultMap, ok := createResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result from create, got %T", createResp.Result)
	}
	sessionID, ok := resultMap["id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected non-empty session id, got %v", resultMap["id"])
	}

	// 6. Send a "subscribe" request for that session.
	subParams, _ := json.Marshal(map[string]string{"id": sessionID})
	subReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "subscribe",
		Params:  json.RawMessage(subParams),
		ID:      2,
	}
	data, _ = json.Marshal(subReq)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read subscribe response: %v", err)
	}

	var subResp JSONRPCResponse
	if err := json.Unmarshal(msg, &subResp); err != nil {
		t.Fatalf("unmarshal subscribe response: %v", err)
	}
	if subResp.Error != nil {
		t.Fatalf("subscribe error: %v", subResp.Error)
	}

	subResult, ok := subResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result from subscribe, got %T", subResp.Result)
	}
	if subResult["subscribed"] != true {
		t.Fatalf("expected subscribed=true, got %v", subResult["subscribed"])
	}
	subID, ok := subResult["sub_id"].(string)
	if !ok || subID == "" {
		t.Fatalf("expected non-empty sub_id, got %v", subResult["sub_id"])
	}

	// 7. Write to the PTY pipe to trigger a screen update.
	if _, err := p.W.WriteString("hello"); err != nil {
		t.Fatalf("write to PTY: %v", err)
	}

	// 8. Read and verify a screen_update notification arrives.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read screen_update notification: %v", err)
	}

	var notif map[string]any
	if err := json.Unmarshal(msg, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif["method"] != "screen_update" {
		t.Errorf("notification method = %v, want screen_update", notif["method"])
	}
	if _, hasID := notif["id"]; hasID {
		t.Error("notification must not have an id field")
	}
	if notif["params"] == nil {
		t.Error("expected non-nil params in screen_update notification")
	}

	// 9. Send "unsubscribe".
	unsubParams, _ := json.Marshal(map[string]string{"id": sessionID, "sub_id": subID})
	unsubReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "unsubscribe",
		Params:  json.RawMessage(unsubParams),
		ID:      3,
	}
	data, _ = json.Marshal(unsubReq)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write unsubscribe: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read unsubscribe response: %v", err)
	}

	var unsubResp JSONRPCResponse
	if err := json.Unmarshal(msg, &unsubResp); err != nil {
		t.Fatalf("unmarshal unsubscribe response: %v", err)
	}
	if unsubResp.Error != nil {
		t.Fatalf("unsubscribe error: %v", unsubResp.Error)
	}

	// 10. Verify no more notifications come after unsubscribe.
	if _, err := p.W.WriteString("world"); err != nil {
		t.Fatalf("write to PTY after unsubscribe: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected no notification after unsubscribe, but received one")
	}
}
