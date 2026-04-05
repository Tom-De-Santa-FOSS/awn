package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestRun_DetectVerboseFlag_ShowsFullStructured(t *testing.T) {
	var gotParams map[string]any
	stdout, err := run([]string{"detect", "sess-123", "--verbose"}, func(_ string, method string, params any) (string, error) {
		if method != "detect" {
			t.Fatalf("method = %q, want detect", method)
		}
		data, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return "", marshalErr
		}
		if unmarshalErr := json.Unmarshal(data, &gotParams); unmarshalErr != nil {
			return "", unmarshalErr
		}
		return `{"elements":[{"type":"button","label":"Save","ref":"button[1]","role":"button","description":"button \"Save\" focused","bounds":{"row":1,"col":2,"width":6,"height":1},"focused":true}],"tree":[{"type":"button","label":"Save","ref":"button[1]","role":"button","description":"button \"Save\" focused","bounds":{"row":1,"col":2,"width":6,"height":1},"focused":true}],"viewport":{"row":0,"col":0,"width":80,"height":24}}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["format"] != "structured" {
		t.Fatalf("format = %#v, want structured", gotParams["format"])
	}
	if !strings.Contains(stdout, "@button[1] [button] \"Save\"") {
		t.Fatalf("stdout = %q, want verbose structured detect text", stdout)
	}
	if !strings.Contains(stdout, "focused") {
		t.Fatalf("stdout = %q, want focused marker", stdout)
	}
}

func TestRun_DetectStructuredFlag_WithJSON_PrintsRawJSON(t *testing.T) {
	stdout, err := run([]string{"--json", "detect", "sess-123", "--structured"}, func(_ string, _ string, _ any) (string, error) {
		return `{"elements":[{"type":"button","label":"Save","ref":"button[1]"}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, `"button[1]"`) {
		t.Fatalf("stdout = %q, want raw JSON", stdout)
	}
}

// --- Current session tests ---

func TestRun_CreateCommand_SetsCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	opts := &runOpts{caller: func(_ string, method string, _ any) (string, error) {
		return `{"id":"abc12345"}`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"create", "bash"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, "current"))
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(data) != "abc12345" {
		t.Fatalf("current = %q, want %q", string(data), "abc12345")
	}
}

func TestRun_CreateCommand_DefaultsDirToWorkingDirectory(t *testing.T) {
	wd := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore Chdir: %v", err)
		}
	})

	var gotParams map[string]any
	_, err = run([]string{"create", "bash"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams) //nolint:errcheck
		return `{"id":"sess-1"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["dir"] != wd {
		t.Fatalf("dir = %v, want %q", gotParams["dir"], wd)
	}
}

func TestRun_ScreenshotCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"rows":24,"cols":80,"lines":["$ "]}`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"screenshot"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_DetectCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"elements":[]}`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"detect"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_CloseCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"close"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_CloseCommand_ClearsCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	opts := &runOpts{caller: func(_ string, _ string, _ any) (string, error) {
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"close"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	_, err = os.ReadFile(filepath.Join(stateDir, "current"))
	if !os.IsNotExist(err) {
		t.Fatalf("current file should be removed after close, err = %v", err)
	}
}

func TestRun_ExplicitID_OverridesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("current-sess"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"rows":24,"cols":80,"lines":["$ "]}`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"screenshot", "--session", "explicit-sess"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "explicit-sess" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "explicit-sess")
	}
}

func TestRun_NoCurrentSession_ErrorsWithoutID(t *testing.T) {
	stateDir := t.TempDir()
	opts := &runOpts{caller: func(_ string, _ string, _ any) (string, error) {
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"screenshot"}, opts)
	if err == nil {
		t.Fatal("expected error when no current session and no ID")
	}
	if !strings.Contains(err.Error(), "no current session") {
		t.Fatalf("error = %q, want 'no current session'", err.Error())
	}
}

func TestRun_PressCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"press", "Enter"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_TypeCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"type", "hello"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_WaitCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `null`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"wait", "--text", "ready"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

// --- Command alias tests ---

func TestRun_OpenCommand_IsAliasForCreate(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"open", "bash"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"id":"abc12345"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "create" {
		t.Fatalf("method = %q, want %q", gotMethod, "create")
	}
	if gotParams["command"] != "bash" {
		t.Fatalf("command = %v, want %q", gotParams["command"], "bash")
	}
	if stdout == "" {
		t.Fatal("expected output")
	}
}

func TestRun_ShowCommand_IsAliasForScreenshot(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	stdout, err := run([]string{"show", "sess-1"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"rows":24,"cols":80,"lines":["$ "]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "screenshot" {
		t.Fatalf("method = %q, want %q", gotMethod, "screenshot")
	}
	if gotParams["id"] != "sess-1" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "sess-1")
	}
	if !strings.Contains(stdout, "$ ") {
		t.Fatalf("stdout = %q, want screen lines", stdout)
	}
}

func TestRun_InspectCommand_IsAliasForDetect(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any

	_, err := run([]string{"inspect", "sess-1"}, func(_ string, method string, params any) (string, error) {
		gotMethod = method
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"elements":[]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotMethod != "detect" {
		t.Fatalf("method = %q, want %q", gotMethod, "detect")
	}
	if gotParams["id"] != "sess-1" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "sess-1")
	}
}

// --- Human-friendly detect rendering tests ---

func TestRun_DetectCommand_DefaultsToHumanReadable(t *testing.T) {
	// Without --json, detect should output human-readable text (not raw JSON)
	stdout, err := run([]string{"detect", "sess-1"}, func(_ string, _ string, params any) (string, error) {
		// Server returns structured data
		return `{"elements":[{"type":"button","label":"Save","ref":"button[1]","role":"button","description":"","bounds":{"row":1,"col":2,"width":6,"height":1},"focused":true}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Should NOT be raw JSON
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatalf("default detect output should be human-readable, got JSON: %q", stdout)
	}
	// Should contain the label
	if !strings.Contains(stdout, "Save") {
		t.Fatalf("stdout = %q, want label 'Save'", stdout)
	}
}

func TestRun_DetectCommand_HumanReadableHidesCoordinates(t *testing.T) {
	stdout, err := run([]string{"detect", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"elements":[{"type":"button","label":"OK","ref":"button[1]","role":"button","bounds":{"row":5,"col":10,"width":4,"height":1},"focused":false}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Human-readable should NOT show coordinates by default
	if strings.Contains(stdout, "@5,10") || strings.Contains(stdout, "5,10") {
		t.Fatalf("human-readable detect should hide coordinates, got %q", stdout)
	}
}

func TestRun_DetectCommand_HumanReadableHidesRefs(t *testing.T) {
	stdout, err := run([]string{"detect", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"elements":[{"type":"button","label":"OK","ref":"button[1]","role":"button","bounds":{"row":5,"col":10,"width":4,"height":1},"focused":false}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Human-readable should NOT show refs by default
	if strings.Contains(stdout, "button[1]") {
		t.Fatalf("human-readable detect should hide refs, got %q", stdout)
	}
}

func TestRun_DetectCommand_HumanReadableShowsFocused(t *testing.T) {
	stdout, err := run([]string{"detect", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"elements":[{"type":"input","label":"Search","ref":"input[1]","role":"textbox","bounds":{"row":0,"col":0,"width":20,"height":1},"focused":true}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "focused") {
		t.Fatalf("human-readable detect should show focused marker, got %q", stdout)
	}
}

func TestRun_DetectCommand_JSONFlagReturnsRawJSON(t *testing.T) {
	stdout, err := run([]string{"--json", "detect", "sess-1"}, func(_ string, _ string, _ any) (string, error) {
		return `{"elements":[{"type":"button","label":"OK","ref":"button[1]"}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// --json should return raw JSON
	if !strings.Contains(stdout, `"ref"`) {
		t.Fatalf("--json should return raw JSON, got %q", stdout)
	}
}

func TestRun_DetectCommand_StructuredFlagRequestsStructured(t *testing.T) {
	var gotParams map[string]any
	_, err := run([]string{"detect", "sess-1", "--structured"}, func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"elements":[{"type":"button","label":"OK","ref":"button[1]","role":"button","bounds":{"row":0,"col":0,"width":4,"height":1},"focused":false}]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["format"] != "structured" {
		t.Fatalf("format = %v, want structured", gotParams["format"])
	}
}

// --- Human-friendly list output tests ---

func TestRun_ListCommand_HumanReadableByDefault(t *testing.T) {
	stdout, err := run([]string{"list"}, func(_ string, _ string, _ any) (string, error) {
		return `{"sessions":["abc12345","def67890"]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Should NOT be raw JSON
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatalf("default list output should be human-readable, got JSON: %q", stdout)
	}
	// Should contain session IDs
	if !strings.Contains(stdout, "abc12345") {
		t.Fatalf("stdout = %q, want session abc12345", stdout)
	}
	if !strings.Contains(stdout, "def67890") {
		t.Fatalf("stdout = %q, want session def67890", stdout)
	}
}

func TestRun_ListCommand_ShowsCurrentMarker(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	opts := &runOpts{caller: func(_ string, _ string, _ any) (string, error) {
		return `{"sessions":["abc12345","def67890"]}`, nil
	}, stateDir: stateDir}

	stdout, err := runWithOpts([]string{"list"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Current session should have a marker
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	foundCurrent := false
	for _, line := range lines {
		if strings.Contains(line, "abc12345") && strings.Contains(line, "*") {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatalf("stdout = %q, want current session marker (*) on abc12345", stdout)
	}
}

func TestRun_ListCommand_EmptyShowsMessage(t *testing.T) {
	stdout, err := run([]string{"list"}, func(_ string, _ string, _ any) (string, error) {
		return `{"sessions":[]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "no sessions") {
		t.Fatalf("stdout = %q, want 'no sessions' message", stdout)
	}
}

func TestRun_ListCommand_JSONFlagReturnsRawJSON(t *testing.T) {
	stdout, err := run([]string{"--json", "list"}, func(_ string, _ string, _ any) (string, error) {
		return `{"sessions":["abc12345"]}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, `"sessions"`) {
		t.Fatalf("--json should return raw JSON, got %q", stdout)
	}
}

func TestRun_ExecCommand_UsesCurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, "current"), []byte("abc12345"), 0o644)

	var gotParams map[string]any
	opts := &runOpts{caller: func(_ string, _ string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams)
		return `{"screen":{"rows":24,"cols":80,"lines":["$ "]}}`, nil
	}, stateDir: stateDir}

	_, err := runWithOpts([]string{"exec", "ls"}, opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["id"] != "abc12345" {
		t.Fatalf("id = %v, want %q", gotParams["id"], "abc12345")
	}
}

func TestRun_CreateWithEnvAndDir(t *testing.T) {
	var gotParams map[string]any
	_, err := run([]string{"create", "bash", "--env", "FOO=bar", "--env", "BAZ=qux", "--dir", "/tmp"}, func(_ string, method string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams) //nolint:errcheck
		return `{"id":"sess-1"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	env, ok := gotParams["env"].(map[string]any)
	if !ok {
		t.Fatalf("env = %T, want map", gotParams["env"])
	}
	if env["FOO"] != "bar" {
		t.Fatalf("env[FOO] = %v", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Fatalf("env[BAZ] = %v", env["BAZ"])
	}
	if gotParams["dir"] != "/tmp" {
		t.Fatalf("dir = %v", gotParams["dir"])
	}
}

func TestRun_CreateWithRecord(t *testing.T) {
	var gotParams map[string]any
	_, err := run([]string{"create", "bash", "--record", "--record-path", "/tmp/demo.cast"}, func(_ string, method string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams) //nolint:errcheck
		return `{"id":"sess-1"}`, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["record"] != true {
		t.Fatalf("record = %v", gotParams["record"])
	}
	if gotParams["record_path"] != "/tmp/demo.cast" {
		t.Fatalf("record_path = %v", gotParams["record_path"])
	}
}

func TestRun_PressWithRepeat(t *testing.T) {
	var gotParams map[string]any
	_, err := run([]string{"press", "sess-1", "Down", "--repeat", "5"}, func(_ string, method string, params any) (string, error) {
		data, _ := json.Marshal(params)
		json.Unmarshal(data, &gotParams) //nolint:errcheck
		return "null", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotParams["repeat"] != float64(5) {
		t.Fatalf("repeat = %v, want 5", gotParams["repeat"])
	}
}

func TestRun_CreateWithEnvBadFormat(t *testing.T) {
	_, err := run([]string{"create", "bash", "--env", "NOEQUALS"}, func(_ string, _ string, _ any) (string, error) {
		return `{"id":"x"}`, nil
	})
	if err == nil {
		t.Fatal("expected error for bad --env format")
	}
	if !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Fatalf("error = %q, want mention of KEY=VALUE", err)
	}
}
