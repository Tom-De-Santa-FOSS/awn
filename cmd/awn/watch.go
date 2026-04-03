package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type watchRPCConn interface {
	WriteJSON(v any) error
	ReadJSON(v any) error
	Close() error
}

type watchedScreen struct {
	Lines []string `json:"lines"`
	State string   `json:"state"`
}

func watch(addr, sessionID string) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Print("\033[2J\033[H")
		fmt.Println("awn watch: disconnected")
		os.Exit(0)
	}()

	err := watchSession(addr, sessionID, dialWatchRPC, time.Sleep, func(screen watchedScreen) error {
		out := renderLines(screen.Lines)
		out += "\n" + renderStatusBar(sessionID, screen.State)
		fmt.Print(out)
		return nil
	})
	if err != nil {
		fatal("watch: " + err.Error())
	}
}

func dialWatchRPC(addr string, header http.Header) (watchRPCConn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func watchSession(addr, sessionID string, dial func(string, http.Header) (watchRPCConn, error), sleep func(time.Duration), render func(watchedScreen) error) error {
	header := http.Header{}
	if token := os.Getenv("AWN_TOKEN"); token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	backoff := 100 * time.Millisecond
	for {
		conn, err := dial(addr, header)
		if err != nil {
			sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		subReq := map[string]any{
			"jsonrpc": "2.0",
			"method":  "subscribe",
			"params":  map[string]any{"id": sessionID},
			"id":      1,
		}
		if err := conn.WriteJSON(subReq); err != nil {
			_ = conn.Close()
			sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		var subResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&subResp); err != nil {
			_ = conn.Close()
			sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}
		if subResp.Error != nil {
			_ = conn.Close()
			return fmt.Errorf("subscribe: %s", subResp.Error.Message)
		}

		for {
			var notif struct {
				Method string        `json:"method"`
				Params watchedScreen `json:"params"`
			}
			if err := conn.ReadJSON(&notif); err != nil {
				_ = conn.Close()
				sleep(backoff)
				backoff = nextBackoff(backoff)
				break
			}
			if notif.Method != "screen_update" {
				continue
			}
			backoff = 100 * time.Millisecond
			if err := render(notif.Params); err != nil {
				_ = conn.Close()
				return err
			}
		}
	}
}

func nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > 2*time.Second {
		return 2 * time.Second
	}
	return current
}
