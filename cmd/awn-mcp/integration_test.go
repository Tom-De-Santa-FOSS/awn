package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tom/awn"
	"github.com/tom/awn/internal/rpc"
)

// integDuplexPTY is a fake PTYStarter that returns a Unix socketpair so the
// session ptmx is both readable (for readLoop) and writable (for SendKeys).
// The test injects data by writing to W.
type integDuplexPTY struct {
	W *os.File // test-side socket; write here to send bytes to the session
}

func (p *integDuplexPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	sessionFile := os.NewFile(uintptr(fds[0]), "pty-session")
	testFile := os.NewFile(uintptr(fds[1]), "pty-test")
	p.W = testFile
	return sessionFile, nil
}

// integStubStrategy is a no-op detection strategy.
type integStubStrategy struct{}

func (integStubStrategy) Detect(screen *awn.Screen) []awn.Element { return nil }

func (integStubStrategy) DetectStructured(screen *awn.Screen) *awn.DetectResult {
	return &awn.DetectResult{
		Elements: []awn.DetectElement{{
			ID:          1,
			Type:        "button",
			Label:       "OK",
			Description: `button "OK"`,
			Role:        "button",
			Ref:         "button[1]",
			Bounds:      awn.Rect{Row: 0, Col: 0, Width: 4, Height: 1},
		}},
		Tree: []awn.DetectTreeNode{{
			ID:          1,
			Type:        "button",
			Label:       "OK",
			Description: `button "OK"`,
			Role:        "button",
			Ref:         "button[1]",
			Bounds:      awn.Rect{Row: 0, Col: 0, Width: 4, Height: 1},
		}},
		Viewport: awn.Rect{Row: 0, Col: 0, Width: 80, Height: 24},
	}
}

// resultText extracts the text content from a CallToolResult.
func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestIntegration_MCP_full_session_lifecycle(t *testing.T) {
	pipe := &integDuplexPTY{}
	driver := awn.NewDriver(awn.WithPTY(pipe))
	handler := rpc.NewHandler(driver, integStubStrategy{})
	s := newServer(handler)
	tools := s.ListTools()

	ctx := context.Background()

	// --- awn_create ---
	createReq := mcp.CallToolRequest{}
	createReq.Params.Name = "awn_create"
	createReq.Params.Arguments = map[string]any{"command": "bash"}

	createResult, err := tools["awn_create"].Handler(ctx, createReq)
	if err != nil {
		t.Fatalf("awn_create: unexpected error: %v", err)
	}
	if createResult == nil || createResult.IsError {
		t.Fatalf("awn_create: expected success, got error result: %v", resultText(createResult))
	}

	// Parse the session ID from the JSON response.
	var createResp struct {
		ID string `json:"id"`
	}
	createText := resultText(createResult)
	if err := json.Unmarshal([]byte(createText), &createResp); err != nil {
		t.Fatalf("awn_create: unmarshal response %q: %v", createText, err)
	}
	sessionID := createResp.ID
	if sessionID == "" {
		t.Fatalf("awn_create: got empty session ID in response: %q", createText)
	}

	// --- awn_list (session should appear) ---
	listReq := mcp.CallToolRequest{}
	listReq.Params.Name = "awn_list"

	listResult, err := tools["awn_list"].Handler(ctx, listReq)
	if err != nil {
		t.Fatalf("awn_list: unexpected error: %v", err)
	}
	if listResult == nil || listResult.IsError {
		t.Fatalf("awn_list: expected success, got error result: %v", resultText(listResult))
	}
	listText := resultText(listResult)
	if !strings.Contains(listText, sessionID) {
		t.Fatalf("awn_list: session %q not found in response: %q", sessionID, listText)
	}

	// --- write to PTY socket, then awn_screenshot ---
	if _, err := pipe.W.WriteString("hello"); err != nil {
		t.Fatalf("write to PTY socket: %v", err)
	}

	// Wait for the text to reach the vt10x emulator.
	sess := driver.Get(sessionID)
	if sess == nil {
		t.Fatalf("driver.Get(%q) returned nil", sessionID)
	}
	if err := sess.WaitForText("hello", time.Second); err != nil {
		t.Fatalf("WaitForText: %v", err)
	}

	screenshotReq := mcp.CallToolRequest{}
	screenshotReq.Params.Name = "awn_screenshot"
	screenshotReq.Params.Arguments = map[string]any{"id": sessionID}

	screenshotResult, err := tools["awn_screenshot"].Handler(ctx, screenshotReq)
	if err != nil {
		t.Fatalf("awn_screenshot: unexpected error: %v", err)
	}
	if screenshotResult == nil || screenshotResult.IsError {
		t.Fatalf("awn_screenshot: expected success, got error result: %v", resultText(screenshotResult))
	}
	screenshotText := resultText(screenshotResult)
	if !strings.Contains(screenshotText, "hello") {
		t.Fatalf("awn_screenshot: expected 'hello' in response, got: %q", screenshotText)
	}

	detectReq := mcp.CallToolRequest{}
	detectReq.Params.Name = "awn_detect"
	detectReq.Params.Arguments = map[string]any{"id": sessionID, "format": "structured"}

	detectResult, err := tools["awn_detect"].Handler(ctx, detectReq)
	if err != nil {
		t.Fatalf("awn_detect: unexpected error: %v", err)
	}
	if detectResult == nil || detectResult.IsError {
		t.Fatalf("awn_detect: expected success, got error result: %v", resultText(detectResult))
	}
	detectText := resultText(detectResult)
	if !strings.Contains(detectText, `"ref":"button[1]"`) {
		t.Fatalf("awn_detect: expected structured detect ref, got: %q", detectText)
	}

	// --- awn_input ---
	inputReq := mcp.CallToolRequest{}
	inputReq.Params.Name = "awn_input"
	inputReq.Params.Arguments = map[string]any{"id": sessionID, "data": "test input"}

	inputResult, err := tools["awn_input"].Handler(ctx, inputReq)
	if err != nil {
		t.Fatalf("awn_input: unexpected error: %v", err)
	}
	if inputResult == nil || inputResult.IsError {
		t.Fatalf("awn_input: expected success, got error result: %v", resultText(inputResult))
	}

	// Close the test-side of the socketpair so readLoop unblocks on ptmx.Close().
	pipe.W.Close()

	// --- awn_close ---
	closeReq := mcp.CallToolRequest{}
	closeReq.Params.Name = "awn_close"
	closeReq.Params.Arguments = map[string]any{"id": sessionID}

	closeResult, err := tools["awn_close"].Handler(ctx, closeReq)
	if err != nil {
		t.Fatalf("awn_close: unexpected error: %v", err)
	}
	if closeResult == nil || closeResult.IsError {
		t.Fatalf("awn_close: expected success, got error result: %v", resultText(closeResult))
	}

	// --- awn_list (session should be gone) ---
	listResult2, err := tools["awn_list"].Handler(ctx, listReq)
	if err != nil {
		t.Fatalf("awn_list (after close): unexpected error: %v", err)
	}
	if listResult2 == nil || listResult2.IsError {
		t.Fatalf("awn_list (after close): expected success, got error result: %v", resultText(listResult2))
	}
	listText2 := resultText(listResult2)
	if strings.Contains(listText2, sessionID) {
		t.Fatalf("awn_list (after close): session %q still present in response: %q", sessionID, listText2)
	}
}
