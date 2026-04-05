package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
)

// mockServer creates a test WebSocket server that responds to JSON-RPC calls.
func mockServer(t *testing.T, handler func(method string, params json.RawMessage) (any, error)) (*httptest.Server, string) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     any             `json:"id"`
		}
		json.Unmarshal(msg, &req) //nolint:errcheck

		result, callErr := handler(req.Method, req.Params)
		var resp map[string]any
		if callErr != nil {
			resp = map[string]any{
				"jsonrpc": "2.0",
				"error":   map[string]any{"code": -32000, "message": callErr.Error()},
				"id":      req.ID,
			}
		} else {
			resp = map[string]any{
				"jsonrpc": "2.0",
				"result":  result,
				"id":      req.ID,
			}
		}
		data, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, data) //nolint:errcheck
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

func TestConnect_DefaultsToUnixSocket(t *testing.T) {
	c, err := Connect()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if c.cfg.socket == "" {
		t.Error("expected default socket path, got empty")
	}
}

func TestConnect_WithAddr(t *testing.T) {
	c, err := Connect(WithAddr("ws://localhost:9999"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if c.cfg.addr != "ws://localhost:9999" {
		t.Errorf("addr = %q", c.cfg.addr)
	}
}

func TestConnect_WithToken(t *testing.T) {
	c, err := Connect(WithAddr("ws://localhost:9999"), WithToken("secret"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if c.cfg.token != "secret" {
		t.Errorf("token = %q", c.cfg.token)
	}
}

func TestPing(t *testing.T) {
	srv, url := mockServer(t, func(method string, _ json.RawMessage) (any, error) {
		if method != "ping" {
			t.Fatalf("method = %q", method)
		}
		return map[string]string{"status": "ok"}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestCreate(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		if method != "create" {
			t.Fatalf("method = %q", method)
		}
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["command"] != "bash" {
			t.Fatalf("command = %v", p["command"])
		}
		return map[string]string{"id": "sess-abc"}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	s, err := c.Create(context.Background(), "bash")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID != "sess-abc" {
		t.Fatalf("ID = %q", s.ID)
	}
}

func TestCreateWithOpts(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["dir"] != "/tmp" {
			t.Fatalf("dir = %v", p["dir"])
		}
		env := p["env"].(map[string]any)
		if env["FOO"] != "bar" {
			t.Fatalf("env[FOO] = %v", env["FOO"])
		}
		if p["record"] != true {
			t.Fatalf("record = %v", p["record"])
		}
		return map[string]string{"id": "sess-1"}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	s, err := c.CreateWithOpts(context.Background(), "bash", CreateOpts{
		Dir:    "/tmp",
		Env:    map[string]string{"FOO": "bar"},
		Record: true,
	})
	if err != nil {
		t.Fatalf("CreateWithOpts: %v", err)
	}
	if s.ID != "sess-1" {
		t.Fatalf("ID = %q", s.ID)
	}
}

func TestScreenshot(t *testing.T) {
	srv, url := mockServer(t, func(method string, _ json.RawMessage) (any, error) {
		return map[string]any{
			"rows": 24, "cols": 80, "hash": "abc123",
			"lines": []string{"$ ls", "file1"},
		}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	scr, err := c.Screenshot(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if scr.Hash != "abc123" {
		t.Fatalf("Hash = %q", scr.Hash)
	}
	if len(scr.Lines) != 2 {
		t.Fatalf("Lines = %v", scr.Lines)
	}
}

func TestScreenshotWithOptions(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["format"] != "full" {
			t.Fatalf("format = %v", p["format"])
		}
		return map[string]any{"rows": 24, "cols": 80, "hash": "x"}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	_, err := c.Screenshot(context.Background(), "sess-1", WithFull())
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
}

func TestType(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["data"] != "hello" {
			t.Fatalf("data = %v", p["data"])
		}
		return nil, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.Type(context.Background(), "sess-1", "hello")
	if err != nil {
		t.Fatalf("Type: %v", err)
	}
}

func TestPress(t *testing.T) {
	var calls int
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		calls++
		return nil, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.Press(context.Background(), "sess-1", "Enter")
	if err != nil {
		t.Fatalf("Press: %v", err)
	}
}

func TestPress_InvalidKey(t *testing.T) {
	c, _ := Connect(WithAddr("ws://localhost:1"))
	err := c.Press(context.Background(), "sess-1", "InvalidKeyName")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	var ae *awn.AwnError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AwnError, got %T: %v", err, err)
	}
	if ae.Code != "INVALID_KEY" {
		t.Fatalf("Code = %q", ae.Code)
	}
}

func TestPressRepeat(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["repeat"] != float64(3) {
			t.Fatalf("repeat = %v", p["repeat"])
		}
		return nil, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.PressRepeat(context.Background(), "sess-1", "Down", 3)
	if err != nil {
		t.Fatalf("PressRepeat: %v", err)
	}
}

func TestExec(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["input"] != "ls -la" {
			t.Fatalf("input = %v", p["input"])
		}
		return map[string]any{
			"screen": map[string]any{"rows": 24, "cols": 80, "hash": "h", "lines": []string{"total 0"}},
		}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	scr, err := c.Exec(context.Background(), "sess-1", "ls -la", WaitStable())
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if scr == nil {
		t.Fatal("expected screen, got nil")
	}
}

func TestWait(t *testing.T) {
	srv, url := mockServer(t, func(method string, params json.RawMessage) (any, error) {
		var p map[string]any
		json.Unmarshal(params, &p) //nolint:errcheck
		if p["text"] != "ready" {
			t.Fatalf("text = %v", p["text"])
		}
		return nil, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.Wait(context.Background(), "sess-1", WaitText("ready"), WithTimeout(10000))
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestClose(t *testing.T) {
	srv, url := mockServer(t, func(method string, _ json.RawMessage) (any, error) {
		if method != "close" {
			t.Fatalf("method = %q", method)
		}
		return nil, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	err := c.Close(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestList(t *testing.T) {
	srv, url := mockServer(t, func(method string, _ json.RawMessage) (any, error) {
		return map[string]any{"sessions": []string{"s1", "s2"}}, nil
	})
	defer srv.Close()

	c, _ := Connect(WithAddr(url))
	resp, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("sessions = %v", resp.Sessions)
	}
}

func TestDisconnect(t *testing.T) {
	c, _ := Connect(WithAddr("ws://localhost:1"))
	if err := c.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestIsRetryable_SDK(t *testing.T) {
	err := awn.ErrConnectionFailed("timeout")
	if !awn.IsRetryable(err) {
		t.Error("connection failed should be retryable")
	}
}

func TestErrorCode_SDK(t *testing.T) {
	err := awn.ErrSessionNotFound("x")
	if awn.ErrorCode(err) != "SESSION_NOT_FOUND" {
		t.Errorf("ErrorCode = %q", awn.ErrorCode(err))
	}
}
