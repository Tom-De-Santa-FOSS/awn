package main

import (
	"encoding/json"
	"fmt"
	"os"
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
			fatal("usage: awn screenshot <session-id> [--json]")
		}
		result := call(addr, "screenshot", map[string]any{"id": os.Args[2]})
		if len(os.Args) > 3 && os.Args[3] == "--json" {
			fmt.Println(result)
		} else {
			// Print just the lines as plain text
			var snap struct {
				Lines []string `json:"lines"`
			}
			json.Unmarshal([]byte(result), &snap)
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

	case "list":
		result := call(addr, "list", nil)
		fmt.Println(result)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func call(addr, method string, params any) string {
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		fatal("connect to daemon: " + err.Error())
	}
	defer conn.Close()

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      1,
	}
	if params != nil {
		req["params"] = params
	}

	data, _ := json.Marshal(req)
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
	json.Unmarshal(msg, &resp)

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
  awn input <id> <text|keys>         Send input to session
  awn wait <id> <text>               Wait for text to appear
  awn close <id>                     Terminate session
  awn list                           List active sessions

Environment:
  AWN_ADDR    Daemon address (default: ws://localhost:7600)`)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
