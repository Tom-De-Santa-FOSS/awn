package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
)

type rpcCaller func(addr, method string, params any) (string, error)

func main() {
	stdout, err := run(os.Args[1:], callRPC)
	if err != nil {
		fatal(err.Error())
	}
	if stdout != "" {
		fmt.Print(stdout)
	}
}

func run(args []string, caller rpcCaller) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: awn <command>")
	}

	addr := os.Getenv("AWN_ADDR")
	if addr == "" {
		addr = "ws://localhost:7600"
	}

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

	switch cmd {
	case "create":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn create <command> [args...]")
		}
		params := map[string]any{"command": args[1], "rows": 24, "cols": 80}
		if len(args) > 2 {
			params["args"] = args[2:]
		}
		result, err := caller(addr, "create", params)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "screenshot":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn screenshot <session-id> [--json] [--diff] [--scrollback N]")
		}
		params := map[string]any{"id": args[1]}
		printJSON := jsonOutput
		for i := 2; i < len(args); i++ {
			switch args[i] {
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
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --scrollback")
				}
				scrollback, err := strconv.Atoi(args[i])
				if err != nil {
					return "", fmt.Errorf("invalid --scrollback value")
				}
				params["scrollback"] = scrollback
				printJSON = true
			default:
				return "", fmt.Errorf("unknown screenshot flag: %s", args[i])
			}
		}
		result, err := caller(addr, "screenshot", params)
		if err != nil {
			return "", err
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
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn press <session-id> <key> [key...]")
		}
		for _, key := range args[2:] {
			seq, ok := awn.ResolveKey(key)
			if !ok {
				return "", fmt.Errorf("unknown key: %s", key)
			}
			_, err := caller(addr, "input", map[string]any{"id": args[1], "data": seq})
			if err != nil {
				return "", err
			}
		}
		return "ok\n", nil
	case "type":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn type <session-id> <text>")
		}
		_, err := caller(addr, "input", map[string]any{"id": args[1], "data": args[2]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "exec":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn exec <session-id> <input> [--timeout <ms>] [--wait-text <text>]")
		}
		params := map[string]any{"id": args[1], "input": args[2]}
		for i := 3; i < len(args); i++ {
			switch args[i] {
			case "--timeout":
				i++
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --timeout")
				}
				ms, err := strconv.Atoi(args[i])
				if err != nil {
					return "", fmt.Errorf("invalid --timeout value: %s", args[i])
				}
				params["timeout_ms"] = ms
			case "--wait-text":
				i++
				if i >= len(args) {
					return "", fmt.Errorf("missing value for --wait-text")
				}
				params["wait_text"] = args[i]
			default:
				return "", fmt.Errorf("unknown exec flag: %s", args[i])
			}
		}
		result, err := caller(addr, "exec", params)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "input":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn input <session-id> <text>")
		}
		_, err := caller(addr, "input", map[string]any{"id": args[1], "data": args[2]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "resize":
		if len(args) < 4 {
			return "", fmt.Errorf("usage: awn resize <session-id> <rows> <cols>")
		}
		rows, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("invalid rows: %s", args[2])
		}
		cols, err := strconv.Atoi(args[3])
		if err != nil {
			return "", fmt.Errorf("invalid cols: %s", args[3])
		}
		_, err = caller(addr, "resize", map[string]any{"id": args[1], "rows": rows, "cols": cols})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "mouse-click":
		if len(args) < 4 {
			return "", fmt.Errorf("usage: awn mouse-click <session-id> <row> <col> [button]")
		}
		row, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("invalid row: %s", args[2])
		}
		col, err := strconv.Atoi(args[3])
		if err != nil {
			return "", fmt.Errorf("invalid col: %s", args[3])
		}
		button := 0
		if len(args) > 4 {
			button, err = strconv.Atoi(args[4])
			if err != nil {
				return "", fmt.Errorf("invalid button: %s", args[4])
			}
		}
		_, err = caller(addr, "mouse_click", map[string]any{"id": args[1], "row": row, "col": col, "button": button})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "mouse-move":
		if len(args) < 4 {
			return "", fmt.Errorf("usage: awn mouse-move <session-id> <row> <col>")
		}
		row, err := strconv.Atoi(args[2])
		if err != nil {
			return "", fmt.Errorf("invalid row: %s", args[2])
		}
		col, err := strconv.Atoi(args[3])
		if err != nil {
			return "", fmt.Errorf("invalid col: %s", args[3])
		}
		_, err = caller(addr, "mouse_move", map[string]any{"id": args[1], "row": row, "col": col})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "wait":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn wait <session-id> [--text <text>] [--stable] [--gone <text>] [--regex <pattern>] [--timeout <ms>]")
		}
		params := map[string]any{"id": args[1]}
		// If args[2] doesn't start with --, treat as backwards-compat positional text
		if len(args) == 3 && !strings.HasPrefix(args[2], "--") {
			params["text"] = args[2]
		} else {
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--text":
					i++
					if i >= len(args) {
						return "", fmt.Errorf("missing value for --text")
					}
					params["text"] = args[i]
				case "--stable":
					params["stable"] = true
				case "--gone":
					i++
					if i >= len(args) {
						return "", fmt.Errorf("missing value for --gone")
					}
					params["gone"] = args[i]
				case "--regex":
					i++
					if i >= len(args) {
						return "", fmt.Errorf("missing value for --regex")
					}
					params["regex"] = args[i]
				case "--timeout":
					i++
					if i >= len(args) {
						return "", fmt.Errorf("missing value for --timeout")
					}
					ms, err := strconv.Atoi(args[i])
					if err != nil {
						return "", fmt.Errorf("invalid --timeout value: %s", args[i])
					}
					params["timeout_ms"] = ms
				default:
					return "", fmt.Errorf("unknown wait flag: %s", args[i])
				}
			}
		}
		_, err := caller(addr, "wait", params)
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "close":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn close <session-id>")
		}
		_, err := caller(addr, "close", map[string]any{"id": args[1]})
		if err != nil {
			return "", err
		}
		return "ok\n", nil
	case "detect":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn detect <session-id>")
		}
		result, err := caller(addr, "detect", map[string]any{"id": args[1]})
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "list":
		result, err := caller(addr, "list", nil)
		if err != nil {
			return "", err
		}
		return result + "\n", nil
	case "record":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn record <session-id> <file>")
		}
		_, err := caller(addr, "record", map[string]any{"id": args[1], "path": args[2]})
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
		if len(args) < 3 {
			return "", fmt.Errorf("usage: awn pipeline <session-id> '<steps-json>'")
		}
		var steps []any
		if err := json.Unmarshal([]byte(args[2]), &steps); err != nil {
			return "", fmt.Errorf("invalid steps JSON: %w", err)
		}
		params := map[string]any{"id": args[1], "steps": steps}
		// Check for optional --stop-on-error flag
		for i := 3; i < len(args); i++ {
			if args[i] == "--stop-on-error" {
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
		return "", fmt.Errorf("unknown command: %s\n\nCommands:\n  create, screenshot, input, press, type, exec, resize,\n  mouse-click, mouse-move, wait, detect, close,\n  list, record, ping, daemon", cmd)
	}
}

func callRPC(addr, method string, params any) (string, error) {
	header := http.Header{}
	if token := os.Getenv("AWN_TOKEN"); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.DefaultDialer.Dial(addr, header)
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
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("%s", resp.Error.Message)
	}

	return string(resp.Result), nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
