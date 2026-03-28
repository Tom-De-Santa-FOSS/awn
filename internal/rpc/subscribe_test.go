package rpc

import (
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/tom/awn"
)

// fakePTY returns a pipe instead of a real PTY for testing.
type fakePTY struct {
	w *os.File
}

func (f *fakePTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	f.w = w
	return r, nil
}

func TestHandler_Subscribe_returns_subscriber_id(t *testing.T) {
	d := awn.NewDriver(awn.WithPTY(&fakePTY{}))
	h := NewHandler(d, stubStrategy{})

	s, err := d.Session("true")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	var notifications []json.RawMessage
	var mu sync.Mutex
	notify := func(data json.RawMessage) {
		mu.Lock()
		notifications = append(notifications, data)
		mu.Unlock()
	}

	subID, err := h.Subscribe(SubscribeRequest{ID: s.ID}, notify)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if subID == "" {
		t.Fatal("expected non-empty subscriber ID")
	}

	h.Unsubscribe(s.ID, subID)
}

func TestHandler_Subscribe_sends_notification_on_screen_update(t *testing.T) {
	p := &fakePTY{}
	d := awn.NewDriver(awn.WithPTY(p))
	h := NewHandler(d, stubStrategy{})

	s, err := d.Session("true")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	got := make(chan json.RawMessage, 1)
	notify := func(data json.RawMessage) {
		select {
		case got <- data:
		default:
		}
	}

	subID, err := h.Subscribe(SubscribeRequest{ID: s.ID}, notify)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer h.Unsubscribe(s.ID, subID)

	// Write to PTY to trigger update.
	_, _ = p.w.WriteString("hello")

	select {
	case data := <-got:
		var resp ScreenResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("unmarshal notification: %v", err)
		}
		if resp.Rows == 0 {
			t.Fatal("expected non-zero rows in notification")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no notification received")
	}
}

func TestHandler_Subscribe_nonexistent_session_returns_error(t *testing.T) {
	d := awn.NewDriver()
	h := NewHandler(d, stubStrategy{})

	_, err := h.Subscribe(SubscribeRequest{ID: "nonexistent"}, func(json.RawMessage) {})
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}
