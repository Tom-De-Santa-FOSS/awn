package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestClient_PingReturnsStatus(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close() //nolint:errcheck

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("Unmarshal request: %v", err)
		}
		if req.Method != "ping" {
			t.Fatalf("method = %q, want %q", req.Method, "ping")
		}
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]any{"status": "ok"},
		}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal response: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http"))
	resp, err := c.Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestClient_RoutesCoreMethods(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close() //nolint:errcheck

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("Unmarshal request: %v", err)
		}
		result := map[string]any{}
		switch req.Method {
		case "create":
			result["id"] = "sess-123"
		case "screenshot":
			result["rows"] = 24
			result["cols"] = 80
			result["lines"] = []string{"hello"}
		case "detect":
			result["elements"] = []map[string]any{{"type": "button", "label": "OK"}}
		case "list":
			result["sessions"] = []string{"sess-123"}
		default:
			result["ok"] = true
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal response: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http"))
	createResp, err := c.Create("bash", "-lc", "echo hi")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if createResp.ID != "sess-123" {
		t.Fatalf("Create ID = %q, want %q", createResp.ID, "sess-123")
	}
	screenshotResp, err := c.Screenshot("sess-123")
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if len(screenshotResp.Lines) != 1 || screenshotResp.Lines[0] != "hello" {
		t.Fatalf("Screenshot lines = %#v, want [\"hello\"]", screenshotResp.Lines)
	}
	if err := c.Input("sess-123", "pwd\n"); err != nil {
		t.Fatalf("Input: %v", err)
	}
	if err := c.Resize("sess-123", 40, 100); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if err := c.Record("sess-123", "/tmp/out.cast"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	detectResp, err := c.Detect("sess-123")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detectResp.Elements) != 1 || detectResp.Elements[0].Label != "OK" {
		t.Fatalf("Detect elements = %#v, want button OK", detectResp.Elements)
	}
	listResp, err := c.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listResp.Sessions) != 1 || listResp.Sessions[0] != "sess-123" {
		t.Fatalf("List sessions = %#v, want [\"sess-123\"]", listResp.Sessions)
	}
	if err := c.Close("sess-123"); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestClient_DetectStructuredReturnsTreeAndRefs(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close() //nolint:errcheck

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("Unmarshal request: %v", err)
		}
		if req.Method != "detect" {
			t.Fatalf("method = %q, want detect", req.Method)
		}
		if req.Params["format"] != "structured" {
			t.Fatalf("format = %#v, want structured", req.Params["format"])
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
			"elements": []map[string]any{{"id": 1, "type": "button", "label": "OK", "ref": "button[1]", "role": "button"}},
			"tree":     []map[string]any{{"id": 1, "type": "button", "label": "OK", "ref": "button[1]", "role": "button"}},
			"viewport": map[string]any{"row": 0, "col": 0, "width": 80, "height": 24},
		}}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal response: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}))
	defer srv.Close()

	c := New("ws" + strings.TrimPrefix(srv.URL, "http"))
	resp, err := c.DetectStructured("sess-123")
	if err != nil {
		t.Fatalf("DetectStructured: %v", err)
	}
	if len(resp.Elements) != 1 || resp.Elements[0].Ref != "button[1]" {
		t.Fatalf("elements = %#v", resp.Elements)
	}
	if len(resp.Tree) != 1 || resp.Tree[0].Role != "button" {
		t.Fatalf("tree = %#v", resp.Tree)
	}
}
