package ws

import (
	"log/slog"
	"testing"
	"time"
)

func TestHub_Broadcast(t *testing.T) {
	h := NewHub(slog.Default())
	go h.Run()
	defer h.Stop()

	c1 := &Client{Send: make(chan []byte, 1)}
	c2 := &Client{Send: make(chan []byte, 1)}
	h.Register(c1)
	h.Register(c2)

	msg := []byte("hello")
	h.Broadcast(msg)

	select {
	case got := <-c1.Send:
		if string(got) != "hello" {
			t.Fatalf("c1 got %q", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting c1")
	}

	select {
	case got := <-c2.Send:
		if string(got) != "hello" {
			t.Fatalf("c2 got %q", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting c2")
	}
}
