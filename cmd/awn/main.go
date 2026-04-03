package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
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
		printJSON := false
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
			return "", fmt.Errorf("usage: awn wait <session-id> <text>")
		}
		_, err := caller(addr, "wait_for_text", map[string]any{"id": args[1], "text": args[2]})
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
	case "watch":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: awn watch <session-id>")
		}
		watch(addr, args[1])
		return "", nil
	default:
		return "", fmt.Errorf("unknown command: %s", cmd)
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
