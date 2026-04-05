package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	awn "github.com/tom/awn"
	"github.com/tom/awn/awtreestrategy"
	"github.com/tom/awn/internal/rpc"
)

// newServer creates an MCP server with all AWN tools registered.
func newServer(d rpc.Dispatcher) *server.MCPServer {
	s := server.NewMCPServer("awn", "0.1.0")

	dispatch := func(_ context.Context, method string, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, err := json.Marshal(req.GetArguments())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, derr := d.Dispatch(method, json.RawMessage(raw))
		if derr != nil {
			return mcp.NewToolResultError(derr.Error()), nil
		}
		out, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	}

	s.AddTool(mcp.NewTool("awn_create",
		mcp.WithDescription("Create a new terminal session"),
		mcp.WithString("command", mcp.Required(), mcp.Description("Command to run")),
		mcp.WithString("args", mcp.Description("Command arguments as JSON array, e.g. [\"--flag\", \"value\"]")),
		mcp.WithNumber("rows", mcp.Description("Terminal rows (default 24)")),
		mcp.WithNumber("cols", mcp.Description("Terminal columns (default 80)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// MCP schema declares args as string; parse it into []string for the RPC handler.
		args := req.GetArguments()
		if raw, ok := args["args"].(string); ok && raw != "" {
			var parsed []string
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				return mcp.NewToolResultError("args must be a JSON array of strings, e.g. [\"--flag\", \"value\"]"), nil
			}
			args["args"] = parsed
		}
		return dispatch(ctx, "create", req)
	})

	s.AddTool(mcp.NewTool("awn_screenshot",
		mcp.WithDescription("Capture the current terminal screen"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("format", mcp.Description("Output format: default, full, structured")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "screenshot", req)
	})

	s.AddTool(mcp.NewTool("awn_input",
		mcp.WithDescription("Send raw input to a terminal session"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("data", mcp.Required(), mcp.Description("Raw text or escape sequence to send")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "input", req)
	})

	s.AddTool(mcp.NewTool("awn_type",
		mcp.WithDescription("Type text into a terminal session (no Enter appended)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to type")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		text, _ := args["text"].(string)
		raw, err := json.Marshal(map[string]any{"id": args["id"], "data": text})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, derr := d.Dispatch("input", json.RawMessage(raw))
		if derr != nil {
			return mcp.NewToolResultError(derr.Error()), nil
		}
		out, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("awn_press",
		mcp.WithDescription("Send named key presses (e.g. Enter, Ctrl+C, ArrowUp)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("keys", mcp.Required(), mcp.Description("Key names as JSON array, e.g. [\"Enter\"] or [\"Ctrl+C\", \"Enter\"]")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		id, _ := args["id"].(string)
		keysRaw, _ := args["keys"].(string)
		var keys []string
		if err := json.Unmarshal([]byte(keysRaw), &keys); err != nil {
			return mcp.NewToolResultError("keys must be a JSON array, e.g. [\"Enter\"]"), nil
		}
		for _, key := range keys {
			seq, ok := awn.ResolveKey(key)
			if !ok {
				return mcp.NewToolResultError("unknown key: " + key), nil
			}
			raw, err := json.Marshal(map[string]any{"id": id, "data": seq})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if _, derr := d.Dispatch("input", json.RawMessage(raw)); derr != nil {
				return mcp.NewToolResultError(derr.Error()), nil
			}
		}
		return mcp.NewToolResultText("ok"), nil
	})

	s.AddTool(mcp.NewTool("awn_exec",
		mcp.WithDescription("Type input, press Enter, and wait for output to stabilize or specific text"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("input", mcp.Required(), mcp.Description("Command or text to type and execute")),
		mcp.WithString("wait_text", mcp.Description("Wait for this text to appear (default: wait for stable screen)")),
		mcp.WithNumber("timeout_ms", mcp.Description("Timeout in milliseconds (default 5000)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "exec", req)
	})

	s.AddTool(mcp.NewTool("awn_wait_for_text",
		mcp.WithDescription("Wait until specific text appears on screen"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to wait for")),
		mcp.WithNumber("timeout_ms", mcp.Description("Timeout in milliseconds (default 5000)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "wait", req)
	})

	s.AddTool(mcp.NewTool("awn_wait_for_stable",
		mcp.WithDescription("Wait until the screen stops changing"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithNumber("timeout_ms", mcp.Description("Timeout in milliseconds (default 5000)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		args["stable"] = true
		raw, err := json.Marshal(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, derr := d.Dispatch("wait", json.RawMessage(raw))
		if derr != nil {
			return mcp.NewToolResultError(derr.Error()), nil
		}
		out, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	s.AddTool(mcp.NewTool("awn_detect",
		mcp.WithDescription("Detect UI elements on the terminal screen"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("format", mcp.Description("Detection output format: flat or structured")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "detect", req)
	})

	s.AddTool(mcp.NewTool("awn_close",
		mcp.WithDescription("Close a terminal session"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "close", req)
	})

	s.AddTool(mcp.NewTool("awn_list",
		mcp.WithDescription("List all active terminal sessions"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "list", req)
	})

	return s
}

func main() {
	driver := awn.NewDriver()
	handler := rpc.NewHandler(driver, awtreestrategy.New())
	s := newServer(handler)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		driver.CloseAll()
		os.Exit(0)
	}()

	if err := server.ServeStdio(s); err != nil {
		os.Exit(1)
	}
}
