package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
)

var version = "dev"
var commit = "none"

type detectRenderElement struct {
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Bounds      awn.Rect `json:"bounds"`
	Focused     bool     `json:"focused"`
	Role        string   `json:"role,omitempty"`
	Ref         string   `json:"ref,omitempty"`
}

type detectRenderResponse struct {
	Elements []detectRenderElement `json:"elements"`
}

type rpcCaller func(addr, method string, params any) (string, error)

// runOpts holds configuration for a CLI invocation.
type runOpts struct {
	caller   rpcCaller
	stateDir string // directory for current-session state; empty disables
}

// defaultStateDir returns ~/.awn for current-session tracking.
func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".awn")
}

// readCurrentSession reads the current session ID from the state directory.
func readCurrentSession(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "current"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeCurrentSession writes the current session ID to the state directory.
func writeCurrentSession(stateDir, id string) {
	if stateDir == "" {
		return
	}
	os.MkdirAll(stateDir, 0o755)
	os.WriteFile(filepath.Join(stateDir, "current"), []byte(id), 0o644)
}

// clearCurrentSession removes the current session file.
func clearCurrentSession(stateDir string) {
	if stateDir == "" {
		return
	}
	os.Remove(filepath.Join(stateDir, "current"))
}

// resolveSessionID resolves a session ID from args or the current session.
// Resolution order:
//  1. --session <id> flag (always consumed from args)
//  2. Current session (if set, positional args are NOT consumed)
//  3. First positional arg (only when no current session, for backwards compat)
//
// Returns the session ID and remaining args.
func resolveSessionID(args []string, stateDir string) (string, []string, error) {
	// Check for explicit --session flag
	for i := 0; i < len(args); i++ {
		if args[i] == "--session" || args[i] == "-s" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for %s", args[i])
			}
			id := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return id, remaining, nil
		}
	}

	// Use current session if set (don't consume positional args)
	current := readCurrentSession(stateDir)
	if current != "" {
		return current, args, nil
	}

	// No current session: consume first non-flag arg as session ID (backwards compat)
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:], nil
	}

	return "", args, fmt.Errorf("no current session; pass a session ID or run 'awn create' first")
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-V" {
			fmt.Printf("awn v%s (%s)\n", version, commit)
			os.Exit(0)
		}
	}
	opts := &runOpts{caller: callRPC, stateDir: defaultStateDir()}
	stdout, err := runWithOpts(os.Args[1:], opts)
	if err != nil {
		fatal(err)
	}
	if stdout != "" {
		fmt.Print(stdout)
	}
}

// run is the original entry point, kept for backward compatibility with existing tests.
func run(args []string, caller rpcCaller) (string, error) {
	return runWithOpts(args, &runOpts{caller: caller})
}

func runWithOpts(args []string, opts *runOpts) (string, error) {
	caller := opts.caller
	if len(args) < 1 {
		return "", fmt.Errorf("usage: awn <command>")
	}

	addr := os.Getenv("AWN_ADDR")
	// If AWN_ADDR is not set, we'll use Unix socket (addr stays empty as signal).
	// AWN_SOCKET or default ~/.awn/daemon.sock is resolved in callRPC.

	// Parse global flags
	jsonOutput := false
	var remaining []string
	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonOutput = true
		default:
			remaining = append(remaining, arg)
		}
	}
	args = remaining
	_ = jsonOutput // used by screenshot to force JSON output

	if len(args) < 1 {
		return "", fmt.Errorf("usage: awn <command>")
	}

	cmd := args[0]

	// Human-friendly command aliases
	switch cmd {
	case "open":
		cmd = "create"
	case "show":
		cmd = "screenshot"
	case "inspect":
		cmd = "detect"
	}

	switch cmd {
	case "create":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn create <command> [args...] [--env KEY=VALUE] [--dir /path] [--record] [--record-path <path>]")
		}
		params := map[string]any{"command": args[1], "rows": 24, "cols": 80}
		var cmdArgs []string
		envVars := map[string]string{}
		record := false
		recordPath := ""
		dir := ""
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--env", "-e":
				i++
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --env")
				}
				parts := strings.SplitN(args[i], "=", 2)
				if len(parts) != 2 {
					return "", fmt.Errorf("invalid --env format, expected KEY=VALUE: %s", args[i])
				}
				envVars[parts[0]] = parts[1]
			case "--dir":
				i++
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --dir")
				}
				dir = args[i]
			case "--record":
				record = true
			case "--record-path":
				i++
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --record-path")
				}
				recordPath = args[i]
				record = true
			default:
				cmdArgs = append(cmdArgs, args[i])
			}
		}
		if len(cmdArgs) > 0 {
			params["args"] = cmdArgs
		}
		if len(envVars) > 0 {
			params["env"] = envVars
		}
		if dir != "" {
			params["dir"] = dir
		}
		if record {
			params["record"] = true
		}
		if recordPath != "" {
			params["record_path"] = recordPath
		}
		result, err := caller(addr, "create", params)
		if err != nil {
			return "", err
		}
		// Save as current session
		var createResp struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(result), &createResp) == nil && createResp.ID != "" {
			writeCurrentSession(opts.stateDir, createResp.ID)
		}
		return result + "\n", nil
	case "screenshot":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn screenshot [session-id] [--json] [--diff] [--scrollback N]\n%w", err)
		}
		params := map[string]any{"id": sessID}
		printJSON := jsonOutput
		for i := 0; i < len(sessArgs); i++ {
			switch sessArgs[i] {
			case "--json":
				printJSON = true
			case "--diff":
				params["format"] = "diff"
				printJSON = true
			case "--full":
				params["format"] = "full"
				printJSON = true
			case "--scrollback":
				i++
				if i >= len(sessArgs) {
					return "", fmt.Errorf("missing value for --scrollback")
				}
				scrollback, err := strconv.Atoi(sessArgs[i])
				if err != nil {
					return "", fmt.Errorf("invalid --scrollback value")
				}
				params["scrollback"] = scrollback
				printJSON = true
			default:
				return "", fmt.Errorf("unknown screenshot flag: %s", sessArgs[i])
			}
		}
		result, callErr := caller(addr, "screenshot", params)
		if callErr != nil {
			return "", callErr
		}
		if printJSON {
			return result + "\n", nil
		}
		var snap struct {
			Lines []string `json:"lines"`
		}
		if err := json.Unmarshal([]byte(result), &snap); err != nil {
			return "", fmt.Errorf("parse screenshot: %w", err)
		}
		return strings.Join(snap.Lines, "\n") + "\n", nil
	case "press":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn press [session-id] <key> [key...] [--repeat N]\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn press [session-id] <key> [key...] [--repeat N]")
		}
		repeat := 1
		var keys []string
		for i := 0; i < len(sessArgs); i++ {
			switch sessArgs[i] {
			case "--repeat", "-n":
				i++
				if i >= len(sessArgs) {
					return "", fmt.Errorf("missing value for --repeat")
				}
				repeat, err = strconv.Atoi(sessArgs[i])
				if err != nil || repeat < 1 {
					return "", fmt.Errorf("invalid --repeat value: %s", sessArgs[i])
				}
			default:
				keys = append(keys, sessArgs[i])
			}
		}
		if len(keys) == 0 {
			return "", fmt.Errorf("usage: awn press [session-id] <key> [key...] [--repeat N]")
		}
		for _, key := range keys {
			seq, ok := awn.ResolveKey(key)
			if !ok {
				return "", awn.ErrInvalidKey(key)
			}
			_, err := caller(addr, "input", map[string]any{"id": sessID, "data": seq, "repeat": repeat})
			if err != nil {
				return "", err
			}
		}
		return "ok\n", nil
	case "type":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn type [session-id] <text>\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn type [session-id] <text>")
		}
		_, err = caller(addr, "input", map[string]any{"id": sessID, "data": sessArgs[0]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "exec":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn exec [session-id] <input> [--timeout <ms>] [--wait-text <text>]\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn exec [session-id] <input> [--timeout <ms>] [--wait-text <text>]")
		}
		params := map[string]any{"id": sessID, "input": sessArgs[0]}
		for i := 1; i < len(sessArgs); i++ {
			switch sessArgs[i] {
			case "--timeout":
				i++
				if i >= len(sessArgs) {
					return "", fmt.Errorf("missing value for --timeout")
				}
				ms, err := strconv.Atoi(sessArgs[i])
				if err != nil {
					return "", fmt.Errorf("invalid --timeout value: %s", sessArgs[i])
				}
				params["timeout_ms"] = ms
			case "--wait-text":
				i++
				if i >= len(sessArgs) {
					return "", fmt.Errorf("missing value for --wait-text")
				}
				params["wait_text"] = sessArgs[i]
			default:
				return "", fmt.Errorf("unknown exec flag: %s", sessArgs[i])
			}
		}
		result, err := caller(addr, "exec", params)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "input":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn input [session-id] <text>\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn input [session-id] <text>")
		}
		_, err = caller(addr, "input", map[string]any{"id": sessID, "data": sessArgs[0]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "resize":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn resize [session-id] <rows> <cols>\n%w", err)
		}
		if len(sessArgs) < 2 {
			return "", fmt.Errorf("usage: awn resize [session-id] <rows> <cols>")
		}
		rows, err := strconv.Atoi(sessArgs[0])
		if err != nil {
			return "", fmt.Errorf("invalid rows: %s", sessArgs[0])
		}
		cols, err := strconv.Atoi(sessArgs[1])
		if err != nil {
			return "", fmt.Errorf("invalid cols: %s", sessArgs[1])
		}
		_, err = caller(addr, "resize", map[string]any{"id": sessID, "rows": rows, "cols": cols})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "mouse-click":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn mouse-click [session-id] <row> <col> [button]\n%w", err)
		}
		if len(sessArgs) < 2 {
			return "", fmt.Errorf("usage: awn mouse-click [session-id] <row> <col> [button]")
		}
		row, err := strconv.Atoi(sessArgs[0])
		if err != nil {
			return "", fmt.Errorf("invalid row: %s", sessArgs[0])
		}
		col, err := strconv.Atoi(sessArgs[1])
		if err != nil {
			return "", fmt.Errorf("invalid col: %s", sessArgs[1])
		}
		button := 0
		if len(sessArgs) > 2 {
			button, err = strconv.Atoi(sessArgs[2])
			if err != nil {
				return "", fmt.Errorf("invalid button: %s", sessArgs[2])
			}
		}
		_, err = caller(addr, "mouse_click", map[string]any{"id": sessID, "row": row, "col": col, "button": button})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "mouse-move":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn mouse-move [session-id] <row> <col>\n%w", err)
		}
		if len(sessArgs) < 2 {
			return "", fmt.Errorf("usage: awn mouse-move [session-id] <row> <col>")
		}
		row, err := strconv.Atoi(sessArgs[0])
		if err != nil {
			return "", fmt.Errorf("invalid row: %s", sessArgs[0])
		}
		col, err := strconv.Atoi(sessArgs[1])
		if err != nil {
			return "", fmt.Errorf("invalid col: %s", sessArgs[1])
		}
		_, err = caller(addr, "mouse_move", map[string]any{"id": sessID, "row": row, "col": col})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "wait":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn wait [session-id] [--text <text>] [--stable] [--gone <text>] [--regex <pattern>] [--timeout <ms>]\n%w", err)
		}
		params := map[string]any{"id": sessID}
		// If single non-flag arg, treat as backwards-compat positional text
		if len(sessArgs) == 1 && !strings.HasPrefix(sessArgs[0], "--") {
			params["text"] = sessArgs[0]
		} else {
			for i := 0; i < len(sessArgs); i++ {
				switch sessArgs[i] {
				case "--text":
					i++
					if i >= len(sessArgs) {
						return "", fmt.Errorf("missing value for --text")
					}
					params["text"] = sessArgs[i]
				case "--stable":
					params["stable"] = true
				case "--gone":
					i++
					if i >= len(sessArgs) {
						return "", fmt.Errorf("missing value for --gone")
					}
					params["gone"] = sessArgs[i]
				case "--regex":
					i++
					if i >= len(sessArgs) {
						return "", fmt.Errorf("missing value for --regex")
					}
					params["regex"] = sessArgs[i]
				case "--timeout":
					i++
					if i >= len(sessArgs) {
						return "", fmt.Errorf("missing value for --timeout")
					}
					ms, err := strconv.Atoi(sessArgs[i])
					if err != nil {
						return "", fmt.Errorf("invalid --timeout value: %s", sessArgs[i])
					}
					params["timeout_ms"] = ms
				default:
					return "", fmt.Errorf("unknown wait flag: %s", sessArgs[i])
				}
			}
		}
		_, err = caller(addr, "wait", params)
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "close":
		sessID, _, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn close [session-id]\n%w", err)
		}
		_, err = caller(addr, "close", map[string]any{"id": sessID})
		if err != nil {
			return "", err
		}
		// Clear current session if we just closed it
		if current := readCurrentSession(opts.stateDir); current == sessID {
			clearCurrentSession(opts.stateDir)
		}
		return "ok\n", nil
	case "detect":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn detect [session-id]\n%w", err)
		}
		params := map[string]any{"id": sessID, "format": "structured"}
		printJSON := jsonOutput
		verbose := false
		for i := 0; i < len(sessArgs); i++ {
			switch sessArgs[i] {
			case "--structured":
				// already default, kept for backwards compat
			case "--json":
				printJSON = true
			case "--verbose", "-v":
				verbose = true
			default:
				return "", fmt.Errorf("unknown detect flag: %s", sessArgs[i])
			}
		}
		result, err := caller(addr, "detect", params)
		if err != nil {
			return "", err
		}
		if printJSON {
			return result + "\n", nil
		}
		if verbose {
			return renderStructuredDetect(result)
		}
		return renderHumanDetect(result)
	case "list":
		result, err := caller(addr, "list", nil)
		if err != nil {
			return "", err
		}
		if jsonOutput {
			return result + "\n", nil
		}
		return renderHumanList(result, readCurrentSession(opts.stateDir))
	case "record":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn record [session-id] <file>\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn record [session-id] <file>")
		}
		_, err = caller(addr, "record", map[string]any{"id": sessID, "path": sessArgs[0]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "ping":
		result, err := caller(addr, "ping", nil)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "pipeline":
		sessID, sessArgs, err := resolveSessionID(args[1:], opts.stateDir)
		if err != nil {
			return "", fmt.Errorf("usage: awn pipeline [session-id] '<steps-json>'\n%w", err)
		}
		if len(sessArgs) < 1 {
			return "", fmt.Errorf("usage: awn pipeline [session-id] '<steps-json>'")
		}
		var steps []any
		if err := json.Unmarshal([]byte(sessArgs[0]), &steps); err != nil {
			return "", fmt.Errorf("invalid steps JSON: %w", err)
		}
		params := map[string]any{"id": sessID, "steps": steps}
		for i := 1; i < len(sessArgs); i++ {
			if sessArgs[i] == "--stop-on-error" {
				params["stop_on_error"] = true
			}
		}
		result, err := caller(addr, "pipeline", params)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "daemon":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn daemon <start|stop|status>")
		}
		return runDaemon(args[1:], addr, caller)
	default:
		return "", fmt.Errorf("unknown command: %s\n\nCommands:\n  open (create), show (screenshot), inspect (detect),\n  press, type, exec, input, resize,\n  mouse-click, mouse-move, wait, close,\n  list, record, ping, pipeline, daemon", cmd)
	}
}

// resolveSocketPath returns the Unix socket path from env or default.
func resolveSocketPath() string {
	if s := os.Getenv("AWN_SOCKET"); s != "" {
		return s
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".awn", "daemon.sock")
}

func callRPC(addr, method string, params any) (string, error) {
	header := http.Header{}
	if token := os.Getenv("AWN_TOKEN"); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	var conn *websocket.Conn
	var err error
	if addr != "" {
		// TCP mode: AWN_ADDR was explicitly set.
		conn, _, err = websocket.DefaultDialer.Dial(addr, header)
	} else {
		// Unix socket mode (default).
		sock := resolveSocketPath()
		dialer := websocket.Dialer{
			NetDial: func(network, a string) (net.Conn, error) {
				return net.Dial("unix", sock)
			},
		}
		conn, _, err = dialer.Dial("ws://localhost/", header)
	}
	if err != nil {
		return "", fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      1,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return "", fmt.Errorf("recv: %w", err)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data,omitempty"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		// Try to parse structured error data.
		if resp.Error.Data != nil {
			var ae awn.AwnError
			if json.Unmarshal(resp.Error.Data, &ae) == nil && ae.Code != "" {
				return "", &ae
			}
		}
		return "", fmt.Errorf("%s", resp.Error.Message)
	}

	return string(resp.Result), nil
}

func renderStructuredDetect(raw string) (string, error) {
	var resp detectRenderResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("parse detect: %w", err)
	}
	if len(resp.Elements) == 0 {
		return "(no elements detected)\n", nil
	}
	lines := make([]string, 0, len(resp.Elements))
	for _, el := range resp.Elements {
		handle := el.Ref
		if handle == "" {
			handle = el.Type
		}
		line := fmt.Sprintf("@%s [%s] %q @%d,%d %dx%d", handle, displayRole(el), el.Label, el.Bounds.Row, el.Bounds.Col, el.Bounds.Width, el.Bounds.Height)
		if el.Focused {
			line += " focused"
		}
		if el.Description != "" {
			line += " - " + el.Description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func renderHumanList(raw string, currentID string) (string, error) {
	var resp struct {
		Sessions []string `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("parse list: %w", err)
	}
	if len(resp.Sessions) == 0 {
		return "(no sessions)\n", nil
	}
	lines := make([]string, 0, len(resp.Sessions))
	for _, id := range resp.Sessions {
		marker := "  "
		if id == currentID {
			marker = "* "
		}
		lines = append(lines, marker+id)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func renderHumanDetect(raw string) (string, error) {
	var resp detectRenderResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("parse detect: %w", err)
	}
	if len(resp.Elements) == 0 {
		return "(no elements detected)\n", nil
	}
	lines := make([]string, 0, len(resp.Elements))
	for _, el := range resp.Elements {
		role := displayRole(el)
		line := fmt.Sprintf("  %s %q", role, el.Label)
		if el.Focused {
			line += " (focused)"
		}
		if el.Description != "" {
			line += " - " + el.Description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func displayRole(el detectRenderElement) string {
	if el.Role != "" {
		return el.Role
	}
	return el.Type
}

func fatal(err error) {
	var ae *awn.AwnError
	if errors.As(err, &ae) {
		fmt.Fprintln(os.Stderr, "error:", ae.Message)
		if ae.Suggestion != "" {
			fmt.Fprintln(os.Stderr, "hint:", ae.Suggestion)
		}
	} else {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}
