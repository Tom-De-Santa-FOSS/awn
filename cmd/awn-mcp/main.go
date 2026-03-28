package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tom/awn"
	"github.com/tom/awn/awtreestrategy"
	"github.com/tom/awn/internal/rpc"
)

// newServer creates an MCP server with all AWN tools registered.
func newServer(d rpc.Dispatcher) *server.MCPServer {
	s := server.NewMCPServer("awn", "0.1.0")

	dispatch := func(ctx context.Context, method string, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		mcp.WithString("args", mcp.Description("Command arguments as JSON array")),
		mcp.WithNumber("rows", mcp.Description("Terminal rows (default 24)")),
		mcp.WithNumber("cols", mcp.Description("Terminal columns (default 80)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		mcp.WithDescription("Send input to a terminal session"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("data", mcp.Required(), mcp.Description("Text or key sequence to send")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "input", req)
	})

	s.AddTool(mcp.NewTool("awn_wait_for_text",
		mcp.WithDescription("Wait until specific text appears on screen"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to wait for")),
		mcp.WithNumber("timeout_ms", mcp.Description("Timeout in milliseconds (default 5000)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "wait_for_text", req)
	})

	s.AddTool(mcp.NewTool("awn_wait_for_stable",
		mcp.WithDescription("Wait until the screen stops changing"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
		mcp.WithNumber("stable_ms", mcp.Description("Stability duration in ms (default 500)")),
		mcp.WithNumber("timeout_ms", mcp.Description("Timeout in milliseconds (default 5000)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return dispatch(ctx, "wait_for_stable", req)
	})

	s.AddTool(mcp.NewTool("awn_detect",
		mcp.WithDescription("Detect UI elements on the terminal screen"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Session ID")),
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
		log.Fatal(err)
	}
}
