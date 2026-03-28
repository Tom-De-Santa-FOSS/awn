package awn

import (
	"testing"
	"time"
)

func TestSession_Subscribe_multiple_subscribers_all_notified(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	id1, ch1 := s.Subscribe()
	defer s.Unsubscribe(id1)
	id2, ch2 := s.Subscribe()
	defer s.Unsubscribe(id2)

	_, _ = p.W.WriteString("x")

	for i, ch := range []<-chan struct{}{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not fire", i)
		}
	}
}

func TestSession_Unsubscribe_stops_delivery(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	id, ch := s.Subscribe()
	s.Unsubscribe(id)

	_, _ = p.W.WriteString("x")
	// Give readLoop a moment to process.
	time.Sleep(50 * time.Millisecond)

	select {
	case <-ch:
		t.Fatal("received notification after unsubscribe")
	default:
		// success — no notification
	}
}

func TestSession_Subscribe_returns_channel_that_fires_on_update(t *testing.T) {
	p := &pipePTY{}
	d := NewDriver(WithPTY(p))

	s, err := d.Session("true")
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	defer d.Close(s.ID) //nolint:errcheck

	id, ch := s.Subscribe()
	defer s.Unsubscribe(id)

	// Write to PTY to trigger an update.
	_, _ = p.W.WriteString("x")

	select {
	case <-ch:
		// success
	case <-time.After(time.Second):
		t.Fatal("subscriber channel did not fire after update")
	}
}
