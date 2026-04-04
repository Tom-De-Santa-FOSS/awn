package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServer_registers_all_tools(t *testing.T) {
	s := newServer(nil)
	tools := s.ListTools()

	want := []string{
		"awn_close",
		"awn_create",
		"awn_detect",
		"awn_exec",
		"awn_input",
		"awn_list",
		"awn_press",
		"awn_screenshot",
		"awn_type",
		"awn_wait_for_stable",
		"awn_wait_for_text",
	}

	if len(tools) != len(want) {
		t.Fatalf("got %d tools, want %d", len(tools), len(want))
	}

	var got []string
	for name := range tools {
		got = append(got, name)
	}
	sort.Strings(got)

	for i, name := range want {
		if got[i] != name {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], name)
		}
	}
}

// fakeDispatcher records the last method+params dispatched.
type fakeDispatcher struct {
	method string
	params json.RawMessage
	result any
	err    error
}

func (f *fakeDispatcher) Dispatch(method string, params json.RawMessage) (any, error) {
	f.method = method
	f.params = params
	return f.result, f.err
}

func TestTool_create_dispatches_with_params(t *testing.T) {
	fake := &fakeDispatcher{
		result: map[string]string{"id": "sess-123"},
	}
	s := newServer(fake)
	tool := s.ListTools()["awn_create"]

	req := mcp.CallToolRequest{}
	req.Params.Name = "awn_create"
	req.Params.Arguments = map[string]any{
		"command": "bash",
		"rows":    float64(40),
		"cols":    float64(120),
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.method != "create" {
		t.Errorf("dispatched to %q, want %q", fake.method, "create")
	}

	// Verify the params were forwarded
	var got map[string]any
	if err := json.Unmarshal(fake.params, &got); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got["command"] != "bash" {
		t.Errorf("command = %v, want %q", got["command"], "bash")
	}

	// Result should contain the session ID
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
}

func TestTool_create_args_string_becomes_array(t *testing.T) {
	fake := &fakeDispatcher{
		result: map[string]string{"id": "sess-456"},
	}
	s := newServer(fake)
	tool := s.ListTools()["awn_create"]

	// MCP sends args as a string (WithString schema), but RPC handler expects []string
	req := mcp.CallToolRequest{}
	req.Params.Name = "awn_create"
	req.Params.Arguments = map[string]any{
		"command": "ls",
		"args":    `["-la", "/tmp"]`,
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(fake.params, &got); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	// args must arrive as a JSON array, not a string
	args, ok := got["args"].([]any)
	if !ok {
		t.Fatalf("args should be array, got %T: %v", got["args"], got["args"])
	}
	if len(args) != 2 || args[0] != "-la" || args[1] != "/tmp" {
		t.Errorf("args = %v, want [-la /tmp]", args)
	}
}

func TestTool_create_args_invalid_json_returns_error(t *testing.T) {
	fake := &fakeDispatcher{
		result: map[string]string{"id": "sess-789"},
	}
	s := newServer(fake)
	tool := s.ListTools()["awn_create"]

	req := mcp.CallToolRequest{}
	req.Params.Name = "awn_create"
	req.Params.Arguments = map[string]any{
		"command": "ls",
		"args":    "not valid json",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler should not return go error, got: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid args JSON")
	}
}

func TestTool_dispatch_error_returns_mcp_error_result(t *testing.T) {
	fake := &fakeDispatcher{
		err: errors.New("session not found"),
	}
	s := newServer(fake)
	tool := s.ListTools()["awn_close"]

	req := mcp.CallToolRequest{}
	req.Params.Name = "awn_close"
	req.Params.Arguments = map[string]any{"id": "nonexistent"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler should not return go error, got: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when dispatch fails")
	}
}

func TestTool_list_dispatches_to_list_method(t *testing.T) {
	fake := &fakeDispatcher{
		result: map[string]any{"sessions": []string{"abc"}},
	}
	s := newServer(fake)
	tools := s.ListTools()
	tool := tools["awn_list"]
	if tool == nil {
		t.Fatal("awn_list tool not found")
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "awn_list"
	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.method != "list" {
		t.Errorf("dispatched to %q, want %q", fake.method, "list")
	}
	// Result should be JSON text
	if result == nil || result.IsError {
		t.Fatalf("expected success result, got error or nil")
	}
}
