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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	addr := os.Getenv("AWN_ADDR")
	if addr == "" {
		addr = "ws://localhost:7600"
	}

	cmd := os.Args[1]

	switch cmd {
	case "create":
		if len(os.Args) < 3 {
			fatal("usage: awn create <command> [args...]")
		}
		params := map[string]any{
			"command": os.Args[2],
			"rows":    24,
			"cols":    80,
		}
		if len(os.Args) > 3 {
			params["args"] = os.Args[3:]
		}
		result := call(addr, "create", params)
		fmt.Println(result)

	case "screenshot":
		if len(os.Args) < 3 {
			fatal("usage: awn screenshot <session-id> [--json] [--diff] [--scrollback N]")
		}
		params := map[string]any{"id": os.Args[2]}
		printJSON := false
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--json":
				printJSON = true
			case "--diff":
				params["format"] = "diff"
				printJSON = true
			case "--scrollback":
				i++
				if i >= len(os.Args) {
					fatal("missing value for --scrollback")
				}
				scrollback, err := strconv.Atoi(os.Args[i])
				if err != nil {
					fatal("invalid --scrollback value")
				}
				params["scrollback"] = scrollback
				printJSON = true
			default:
				fatal("unknown screenshot flag: " + os.Args[i])
			}
		}
		result := call(addr, "screenshot", params)
		if printJSON {
			fmt.Println(result)
		} else {
			// Print just the lines as plain text
			var snap struct {
				Lines []string `json:"lines"`
			}
			if err := json.Unmarshal([]byte(result), &snap); err != nil {
				fatal("parse screenshot: " + err.Error())
			}
			fmt.Println(strings.Join(snap.Lines, "\n"))
		}

	case "input":
		if len(os.Args) < 4 {
			fatal("usage: awn input <session-id> <text>")
		}
		call(addr, "input", map[string]any{
			"id":   os.Args[2],
			"data": os.Args[3],
		})
		fmt.Println("ok")

	case "mouse-click":
		if len(os.Args) < 5 {
			fatal("usage: awn mouse-click <session-id> <row> <col> [button]")
		}
		row := mustInt(os.Args[3], "row")
		col := mustInt(os.Args[4], "col")
		button := 0
		if len(os.Args) > 5 {
			button = mustInt(os.Args[5], "button")
		}
		call(addr, "mouse_click", map[string]any{"id": os.Args[2], "row": row, "col": col, "button": button})
		fmt.Println("ok")

	case "mouse-move":
		if len(os.Args) < 5 {
			fatal("usage: awn mouse-move <session-id> <row> <col>")
		}
		row := mustInt(os.Args[3], "row")
		col := mustInt(os.Args[4], "col")
		call(addr, "mouse_move", map[string]any{"id": os.Args[2], "row": row, "col": col})
		fmt.Println("ok")

	case "wait":
		if len(os.Args) < 4 {
			fatal("usage: awn wait <session-id> <text>")
		}
		call(addr, "wait_for_text", map[string]any{
			"id":   os.Args[2],
			"text": os.Args[3],
		})
		fmt.Println("ok")

	case "close":
		if len(os.Args) < 3 {
			fatal("usage: awn close <session-id>")
		}
		call(addr, "close", map[string]any{"id": os.Args[2]})
		fmt.Println("ok")

	case "detect":
		if len(os.Args) < 3 {
			fatal("usage: awn detect <session-id>")
		}
		result := call(addr, "detect", map[string]any{"id": os.Args[2]})
		fmt.Println(result)

	case "list":
		result := call(addr, "list", nil)
		fmt.Println(result)

	case "record":
		if len(os.Args) < 4 {
			fatal("usage: awn record <session-id> <file>")
		}
		call(addr, "record", map[string]any{"id": os.Args[2], "path": os.Args[3]})
		fmt.Println("ok")

	case "watch":
		if len(os.Args) < 3 {
			fatal("usage: awn watch <session-id>")
		}
		watch(addr, os.Args[2])

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func call(addr, method string, params any) string {
	header := http.Header{}
	if token := os.Getenv("AWN_TOKEN"); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		fatal("connect to daemon: " + err.Error())
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
		fatal("marshal request: " + err.Error())
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		fatal("send: " + err.Error())
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		fatal("recv: " + err.Error())
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		fatal("parse response: " + err.Error())
	}

	if resp.Error != nil {
		fatal(resp.Error.Message)
	}

	return string(resp.Result)
}

func usage() {
	fmt.Fprintln(os.Stderr, `awn — TUI automation for AI agents

Commands:
  awn create <command> [args...]     Start a TUI session
  awn screenshot <id> [--json]       Capture screen state
  awn screenshot <id> --diff         Return changed rows since last screenshot
  awn detect <id>                    Detect UI elements (accessibility tree)
  awn input <id> <text|keys>         Send input to session
  awn mouse-click <id> <row> <col> [button]
                                     Send xterm mouse click
  awn mouse-move <id> <row> <col>    Send xterm mouse move
  awn wait <id> <text>               Wait for text to appear
  awn record <id> <file>             Write asciicast v2 recording
  awn close <id>                     Terminate session
  awn list                           List active sessions
  awn watch <id>                     Watch session screen in real-time

Environment:
  AWN_ADDR    Daemon address (default: ws://localhost:7600)
  AWN_TOKEN   Bearer token for authentication`)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func mustInt(value, name string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fatal("invalid " + name + ": " + value)
	}
	return parsed
}
