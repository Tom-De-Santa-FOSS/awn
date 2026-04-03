package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

type fakeWatchConn struct {
	reads  [][]byte
	errs   []error
	writes [][]byte
}

func (f *fakeWatchConn) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f.writes = append(f.writes, data)
	return nil
}

func (f *fakeWatchConn) ReadJSON(v any) error {
	if len(f.reads) == 0 {
		if len(f.errs) == 0 {
			return io.EOF
		}
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	data := f.reads[0]
	f.reads = f.reads[1:]
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		if err != nil {
			return err
		}
	}
	return json.Unmarshal(data, v)
}

func (f *fakeWatchConn) Close() error { return nil }

func TestWatchSession_ReconnectsAndResubscribesAfterReadError(t *testing.T) {
	subscribeResp := []byte(`{"result":{"sub_id":"sub-1"}}`)
	notif := []byte(`{"method":"screen_update","params":{"lines":["ready"],"state":"idle"}}`)

	conns := []*fakeWatchConn{
		{reads: [][]byte{subscribeResp}, errs: []error{nil, io.ErrUnexpectedEOF}},
		{reads: [][]byte{subscribeResp, notif}, errs: []error{nil, nil}},
	}
	dials := 0
	sleeps := 0
	var renderedLines []string

	errStop := errors.New("stop")
	err := watchSession(
		"ws://example",
		"sess-123",
		func(_ string, _ http.Header) (watchRPCConn, error) {
			if dials >= len(conns) {
				return nil, io.EOF
			}
			conn := conns[dials]
			dials++
			return conn, nil
		},
		func(_ time.Duration) { sleeps++ },
		func(screen watchedScreen) error {
			renderedLines = append(renderedLines, screen.Lines...)
			return errStop
		},
	)
	if !errors.Is(err, errStop) {
		t.Fatalf("watchSession error = %v, want %v", err, errStop)
	}
	if dials != 2 {
		t.Fatalf("dial count = %d, want %d", dials, 2)
	}
	if sleeps != 1 {
		t.Fatalf("sleep count = %d, want %d", sleeps, 1)
	}
	if len(renderedLines) != 1 || renderedLines[0] != "ready" {
		t.Fatalf("rendered lines = %#v, want [\"ready\"]", renderedLines)
	}
	for i, conn := range conns {
		if len(conn.writes) != 1 {
			t.Fatalf("conn %d writes = %d, want 1 subscribe request", i, len(conn.writes))
		}
	}
}
