package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
)

func watch(addr, sessionID string) {
	header := http.Header{}
	if token := os.Getenv("AWN_TOKEN"); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		fatal("connect to daemon: " + err.Error())
	}
	defer conn.Close() //nolint:errcheck

	// Subscribe to screen updates.
	subReq := map[string]any{
		"jsonrpc": "2.0",
		"method":  "subscribe",
		"params":  map[string]any{"id": sessionID},
		"id":      1,
	}
	data, _ := json.Marshal(subReq)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		fatal("send subscribe: " + err.Error())
	}

	// Read subscribe response.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fatal("read subscribe response: " + err.Error())
	}

	var subResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &subResp); err != nil {
		fatal("parse subscribe response: " + err.Error())
	}
	if subResp.Error != nil {
		fatal("subscribe: " + subResp.Error.Message)
	}

	// Parse sub_id for unsubscribe.
	var subResult struct {
		SubID string `json:"sub_id"`
	}
	_ = json.Unmarshal(subResp.Result, &subResult)

	// Handle Ctrl+C: unsubscribe and exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		unsubReq := map[string]any{
			"jsonrpc": "2.0",
			"method":  "unsubscribe",
			"params":  map[string]any{"id": sessionID, "sub_id": subResult.SubID},
			"id":      2,
		}
		data, _ := json.Marshal(unsubReq)
		_ = conn.WriteMessage(websocket.TextMessage, data)
		// Clear screen and show exit message.
		fmt.Print("\033[2J\033[H")
		fmt.Println("awn watch: disconnected")
		os.Exit(0)
	}()

	// Render loop: read notifications and render.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fatal("read: " + err.Error())
		}

		var notif struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(msg, &notif); err != nil {
			continue
		}
		if notif.Method != "screen_update" {
			continue
		}

		var screen struct {
			Lines []string `json:"lines"`
			State string   `json:"state"`
		}
		if err := json.Unmarshal(notif.Params, &screen); err != nil {
			continue
		}

		out := renderLines(screen.Lines)
		out += "\n" + renderStatusBar(sessionID, screen.State)
		fmt.Print(out)
	}
}
