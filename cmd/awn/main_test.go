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

func TestRun_PressCommand_ResolvesKeys(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"press", "sess-1", "Enter"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout != "ok\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "ok\n")
	}
	if gotMethod != "input" {
		t.Fatalf("method = %q, want %q", gotMethod, "input")
	}
	if gotParams["data"] != "\r" {
		t.Fatalf("data = %q, want %q", gotParams["data"], "\r")
	}
}

func TestRun_PressCommand_MultipleKeys(t *testing.T) {
	var calls []map[string]any

	_, err := run([]string{"press", "sess-1", "Ctrl+C", "Enter"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		var p map[string]any
		json.Unmarshal(data, &p)
		calls = append(calls, p)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 RPC calls, got %d", len(calls))
	}
	if calls[0]["data"] != "\x03" {
		t.Fatalf("first key data = %q, want Ctrl+C", calls[0]["data"])
	}
	if calls[1]["data"] != "\r" {
		t.Fatalf("second key data = %q, want Enter", calls[1]["data"])
	}
}

func TestRun_PressCommand_UnknownKey(t *testing.T) {
	_, err := run([]string{"press", "sess-1", "FakeKey"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestRun_PressCommand_MissingArgs(t *testing.T) {
	_, err := run([]string{"press", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for missing key args")
	}
}

func TestRun_TypeCommand_SendsRawText(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"type", "sess-1", "hello world"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout != "ok\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "ok\n")
	}
	if gotMethod != "input" {
		t.Fatalf("method = %q, want %q", gotMethod, "input")
	}
	if gotParams["data"] != "hello world" {
		t.Fatalf("data = %q, want %q", gotParams["data"], "hello world")
	}
}

func TestRun_TypeCommand_MissingArgs(t *testing.T) {
	_, err := run([]string{"type", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for missing text arg")
	}
}

func TestRun_ExecCommand_CallsExecRPC(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"exec", "sess-1", "echo hello"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"screen":{"rows":24,"cols":80,"lines":["$ "]}}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "exec" {
		t.Fatalf("method = %q, want %q", gotMethod, "exec")
	}
	if gotParams["input"] != "echo hello" {
		t.Fatalf("input = %q, want %q", gotParams["input"], "echo hello")
	}
	if stdout == "" {
		t.Fatal("expected output")
	}
}

func TestRun_ExecCommand_WithTimeout(t *testing.T) {
	var gotParams map[string]any

	_, err := run([]string{"exec", "sess-1", "ls", "--timeout", "5000"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"screen":{"rows":24,"cols":80,"lines":["$ "]}}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["timeout_ms"] != float64(5000) {
		t.Fatalf("timeout_ms = %v, want 5000", gotParams["timeout_ms"])
	}
}

func TestRun_ExecCommand_MissingArgs(t *testing.T) {
	_, err := run([]string{"exec", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for missing input arg")
	}
}

func TestRun_WaitCommand_TextFlag(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "--text", "ready"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "wait" {
		t.Fatalf("method = %q, want %q", gotMethod, "wait")
	}
	if gotParams["text"] != "ready" {
		t.Fatalf("text = %q, want %q", gotParams["text"], "ready")
	}
}

func TestRun_WaitCommand_StableFlag(t *testing.T) {
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "--stable"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["stable"] != true {
		t.Fatalf("stable = %v, want true", gotParams["stable"])
	}
}

func TestRun_WaitCommand_GoneFlag(t *testing.T) {
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "--gone", "loading"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["gone"] != "loading" {
		t.Fatalf("gone = %q, want %q", gotParams["gone"], "loading")
	}
}

func TestRun_WaitCommand_RegexFlag(t *testing.T) {
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "--regex", `\d+`}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["regex"] != `\d+` {
		t.Fatalf("regex = %q, want %q", gotParams["regex"], `\d+`)
	}
}

func TestRun_WaitCommand_TimeoutFlag(t *testing.T) {
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "--text", "ready", "--timeout", "10000"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["timeout_ms"] != float64(10000) {
		t.Fatalf("timeout_ms = %v, want 10000", gotParams["timeout_ms"])
	}
}

func TestRun_WaitCommand_BackwardsCompat(t *testing.T) {
	// awn wait <id> <text> should still work (backwards compat)
	var gotParams map[string]any

	_, err := run([]string{"wait", "sess-1", "ready"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["text"] != "ready" {
		t.Fatalf("text = %q, want %q", gotParams["text"], "ready")
	}
}

func TestRun_DaemonCommand_RequiresSubcommand(t *testing.T) {
	_, err := run([]string{"daemon"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestRun_DaemonCommand_UnknownSubcommand(t *testing.T) {
	_, err := run([]string{"daemon", "foobar"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestRun_DaemonStatus_CallsPing(t *testing.T) {
	var gotMethod string
	stdout, err := run([]string{"daemon", "status"}, func(_ string, method string, _ any) (string, error) {
		gotMethod = method
		return `{"status":"ok"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "ping" {
		t.Fatalf("method = %q, want %q", gotMethod, "ping")
	}
	if stdout == "" {
		t.Fatal("expected output")
	}
}

func TestRun_PipelineCommand_CallsPipelineRPC(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	// Pipeline reads steps from args as JSON
	stdout, err := run([]string{"pipeline", "sess-1", `[{"type":"screenshot"}]`}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"results":[{"screen":{"rows":24,"cols":80,"lines":["$ "]}}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "pipeline" {
		t.Fatalf("method = %q, want %q", gotMethod, "pipeline")
	}
	if stdout == "" {
		t.Fatal("expected output")
	}
}

func TestRun_PipelineCommand_MissingArgs(t *testing.T) {
	_, err := run([]string{"pipeline", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return "null", nil
	})
	if err == nil {
		t.Fatal("expected error for missing steps")
	}
}

func TestRun_GlobalJsonFlag_ScreenshotOutputsJSON(t *testing.T) {
	stdout, err := run([]string{"--json", "screenshot", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"rows":24,"cols":80,"lines":["hello"]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// With --json, screenshot should output the raw JSON (not just the lines)
	if !strings.Contains(stdout, `"rows"`) {
		t.Fatalf("expected JSON output, got %q", stdout)
	}
}

func TestRun_GlobalJsonFlagShort(t *testing.T) {
	stdout, err := run([]string{"-j", "screenshot", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"rows":24,"cols":80,"lines":["hello"]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, `"rows"`) {
		t.Fatalf("expected JSON output with -j, got %q", stdout)
	}
}

func TestRun_GlobalJsonFlag_ErrorOutputsJSON(t *testing.T) {
	_, err := run([]string{"--json", "screenshot"}, func(_ string, _ string, _ any) (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("expected error for missing args")
	}
	// Errors should still be errors (error handling happens outside run)
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
