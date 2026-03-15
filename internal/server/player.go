package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"sync/atomic"

	"github.com/kiptoon/tictactoe/internal/wire"
)

type Player struct {
	// name is the display name announced during the hello handshake.
	name string
	// symbol is assigned by a session when the player is matched.
	symbol string
	peer   wire.Conn
	// msgs carries decoded client messages to the owning session.
	msgs chan wire.Message
	// closed is closed exactly once when the player disconnects.
	closed chan struct{}
	done   atomic.Bool
}

// NewPlayer wraps a client connection together with the session-facing state the server needs.
func NewPlayer(name string, conn net.Conn) *Player {
	return &Player{
		name:   name,
		peer:   wire.NewJSONConn(conn),
		msgs:   make(chan wire.Message, 16),
		closed: make(chan struct{}),
	}
}

func (p *Player) Name() string {
	return p.name
}

func (p *Player) Symbol() string {
	return p.symbol
}

func (p *Player) SetSymbol(symbol string) {
	p.symbol = symbol
}

func (p *Player) Messages() <-chan wire.Message {
	return p.msgs
}

func (p *Player) Closed() <-chan struct{} {
	return p.closed
}

func (p *Player) Send(msg wire.Message) error {
	return p.peer.Send(msg)
}

func (p *Player) ReadLoop(decoder *json.Decoder) {
	defer p.Close()

	for {
		var msg wire.Message
		if err := decoder.Decode(&msg); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("[server] read error for player %q: %v", p.name, err)
			}
			return
		}

		select {
		// Sessions consume moves from this channel; if the player has already been closed we stop.
		case p.msgs <- msg:
		case <-p.closed:
			return
		}
	}
}

func (p *Player) Close() {
	if p.done.CompareAndSwap(false, true) {
		close(p.closed)
		_ = p.peer.Close()
	}
}

func (p *Player) Alive() bool {
	return !p.done.Load()
}
