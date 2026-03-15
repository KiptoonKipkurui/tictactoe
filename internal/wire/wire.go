package wire

import (
	"encoding/json"
	"net"
	"sync"
)

type Message struct {
	// Type identifies the semantic meaning of the message, for example hello, move, state, info, error.
	Type     string    `json:"type"`
	Name     string    `json:"name,omitempty"`
	Row      int       `json:"row,omitempty"`
	Col      int       `json:"col,omitempty"`
	Board    [9]string `json:"board,omitempty"`
	Symbol   string    `json:"symbol,omitempty"`
	Turn     string    `json:"turn,omitempty"`
	Status   string    `json:"status,omitempty"`
	Text     string    `json:"text,omitempty"`
	Opponent string    `json:"opponent,omitempty"`
	Winner   string    `json:"winner,omitempty"`
}

// Conn is the minimal transport surface the client and server need from a wire connection.
type Conn interface {
	Send(Message) error
	Close() error
}

// JSONConn serializes writes so concurrent goroutines can safely send JSON messages on one connection.
type JSONConn struct {
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

// NewJSONConn wraps a net.Conn with JSON message encoding.
func NewJSONConn(conn net.Conn) Conn {
	return &JSONConn{
		conn: conn,
		enc:  json.NewEncoder(conn),
	}
}

func (c *JSONConn) Send(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(msg)
}

func (c *JSONConn) Close() error {
	return c.conn.Close()
}
