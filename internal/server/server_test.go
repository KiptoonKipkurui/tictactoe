package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kiptoon/tictactoe/internal/wire"
)

func TestHandleConnQueuesPlayerAfterHello(t *testing.T) {
	srv := New(":0")
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(context.Background(), serverConn)
	}()

	if err := json.NewEncoder(clientConn).Encode(wire.Message{Type: "hello", Name: " Alice "}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	msg := mustDecodeWireMessage(t, clientConn)
	if msg.Type != "info" || msg.Status != "waiting" {
		t.Fatalf("server info = %+v, want waiting info", msg)
	}

	select {
	case player := <-srv.waiting:
		if got := player.Name(); got != "Alice" {
			t.Fatalf("queued player name = %q, want %q", got, "Alice")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued player")
	}

	_ = clientConn.Close()
	<-done
}

func TestHandleConnRejectsNonHelloFirstMessage(t *testing.T) {
	srv := New(":0")
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConn(context.Background(), serverConn)
	}()

	if err := json.NewEncoder(clientConn).Encode(wire.Message{Type: "move", Row: 0, Col: 0}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	msg := mustDecodeWireMessage(t, clientConn)
	if msg.Type != "error" || msg.Text != "first message must be of type hello" {
		t.Fatalf("server error = %+v, want handshake error", msg)
	}

	buf := make([]byte, 1)
	if _, err := clientConn.Read(buf); err != io.EOF {
		t.Fatalf("Read() error = %v, want EOF after rejection", err)
	}

	select {
	case player := <-srv.waiting:
		t.Fatalf("unexpected queued player: %s", player.Name())
	case <-time.After(200 * time.Millisecond):
	}

	<-done
}

func TestHandleConnDefaultsBlankNameToAnonymous(t *testing.T) {
	srv := New(":0")
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go srv.handleConn(context.Background(), serverConn)

	if err := json.NewEncoder(clientConn).Encode(wire.Message{Type: "hello", Name: "   "}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	_ = mustDecodeWireMessage(t, clientConn)

	select {
	case player := <-srv.waiting:
		if got := player.Name(); got != "Anonymous" {
			t.Fatalf("queued player name = %q, want %q", got, "Anonymous")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued player")
	}
}

func mustDecodeWireMessage(t *testing.T, conn net.Conn) wire.Message {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	defer conn.SetReadDeadline(time.Time{})

	var msg wire.Message
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	return msg
}
