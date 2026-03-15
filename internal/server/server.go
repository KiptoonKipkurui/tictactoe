package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/kiptoon/tictactoe/internal/wire"
)

type Server struct {
	addr string
	// waiting is the shared lobby queue consumed by the matchmaker.
	waiting chan *Player

	mu sync.Mutex
	// players tracks all live connections so shutdown can close them proactively.
	players map[*Player]struct{}
	wg      sync.WaitGroup
}

// New creates the TCP server and its shared waiting queue.
func New(addr string) *Server {
	return &Server{
		addr:    addr,
		waiting: make(chan *Player, 128),
		players: make(map[*Player]struct{}),
	}
}

// Run accepts connections, performs the initial handshake, and feeds players into matchmaking.
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	log.Printf("[server] listening on %s", s.addr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		// Closing all player connections causes their read loops and sessions to unwind naturally.
		s.closeAllPlayers()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		NewMatchmaker(s.waiting, &s.wg).Run(ctx)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				s.wg.Wait()
				return nil
			}
			log.Printf("[server] accept error: %v", err)
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, conn)
		}()
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	decoder := json.NewDecoder(conn)

	var hello wire.Message
	if err := decoder.Decode(&hello); err != nil {
		log.Printf("[server] handshake read failed: %v", err)
		_ = conn.Close()
		return
	}

	if hello.Type != "hello" {
		_ = wire.NewJSONConn(conn).Send(wire.Message{
			Type: "error",
			Text: "first message must be of type hello",
		})
		_ = conn.Close()
		return
	}

	name := strings.TrimSpace(hello.Name)
	if name == "" {
		name = "Anonymous"
	}

	player := NewPlayer(name, conn)
	s.registerPlayer(player)
	log.Printf("[server] client connected: %s", player.Name())
	_ = player.Send(wire.Message{
		Type:   "info",
		Status: "waiting",
		Text:   "Connected. Waiting for an opponent...",
	})

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		player.ReadLoop(decoder)
		s.unregisterPlayer(player)
	}()

	select {
	case <-ctx.Done():
		player.Close()
	case s.waiting <- player:
	}
}

func (s *Server) registerPlayer(player *Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.players[player] = struct{}{}
}

func (s *Server) unregisterPlayer(player *Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.players, player)
}

func (s *Server) closeAllPlayers() {
	s.mu.Lock()
	players := make([]*Player, 0, len(s.players))
	for player := range s.players {
		players = append(players, player)
	}
	s.mu.Unlock()

	for _, player := range players {
		player.Close()
	}
}
