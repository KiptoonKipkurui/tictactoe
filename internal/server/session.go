package server

import (
	"context"
	"fmt"
	"log"

	gamepkg "github.com/kiptoon/tictactoe/internal/game"
	"github.com/kiptoon/tictactoe/internal/wire"
)

type Session struct {
	// ctx is canceled when the server is shutting down.
	ctx context.Context
	// game holds the authoritative rules and state for this match.
	game gamepkg.Engine
	// waiting is used to send surviving players back to the lobby after a match ends.
	waiting chan<- *Player
	xPlayer *Player
	oPlayer *Player
}

// NewSession builds one authoritative match between two connected players.
func NewSession(ctx context.Context, waiting chan<- *Player, xPlayer, oPlayer *Player) *Session {
	return &Session{
		ctx:     ctx,
		game:    gamepkg.New(),
		waiting: waiting,
		xPlayer: xPlayer,
		oPlayer: oPlayer,
	}
}

// Run drives a single match until it ends by win, draw, or disconnect.
func (s *Session) Run() {
	s.xPlayer.SetSymbol("X")
	s.oPlayer.SetSymbol("O")

	log.Printf("[session] started: %s (X) vs %s (O)", s.xPlayer.Name(), s.oPlayer.Name())

	s.sendState(s.xPlayer, s.oPlayer, "Game started. You play first.")
	s.sendState(s.oPlayer, s.xPlayer, "Game started. Wait for your turn.")

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.xPlayer.Closed():
			s.endByDisconnect(s.oPlayer, s.xPlayer)
			return
		case <-s.oPlayer.Closed():
			s.endByDisconnect(s.xPlayer, s.oPlayer)
			return
		case msg := <-s.xPlayer.Messages():
			if !s.handleMessage(s.xPlayer, s.oPlayer, msg) {
				return
			}
		case msg := <-s.oPlayer.Messages():
			if !s.handleMessage(s.oPlayer, s.xPlayer, msg) {
				return
			}
		}
	}
}

func (s *Session) handleMessage(actor, opponent *Player, msg wire.Message) bool {
	if msg.Type != "move" {
		_ = actor.Send(wire.Message{
			Type: "error",
			Text: "unsupported message type",
		})
		return true
	}

	if err := s.game.ApplyMove(msg.Row, msg.Col, actor.Symbol()); err != nil {
		_ = actor.Send(wire.Message{
			Type: "error",
			Text: err.Error(),
		})
		return true
	}

	if winner := s.game.Winner(); winner != "" {
		// The acting player made the winning move, so they receive the winner-relative result.
		s.sendFinished(actor, opponent, winner)
		s.requeue(actor, opponent)
		return false
	}

	if s.game.Draw() {
		s.sendDraw(actor, opponent)
		s.requeue(actor, opponent)
		return false
	}

	s.sendState(actor, opponent, "Move accepted. Waiting for opponent.")
	s.sendState(opponent, actor, fmt.Sprintf("%s made a move. Your turn.", actor.Name()))
	return true
}

func (s *Session) sendState(recipient, opponent *Player, text string) {
	snapshot := s.game.Snapshot()
	if snapshot.Turn == recipient.Symbol() {
		text += " Your move."
	}

	_ = recipient.Send(wire.Message{
		Type:     "state",
		Board:    snapshot.Board,
		Symbol:   recipient.Symbol(),
		Turn:     snapshot.Turn,
		Status:   "playing",
		Text:     text,
		Opponent: opponent.Name(),
	})
}

// sendFinished customizes the result text so each client sees a player-relative message.
func (s *Session) sendFinished(winner, loser *Player, winnerSymbol string) {
	snapshot := s.game.Snapshot()

	_ = winner.Send(wire.Message{
		Type:     "state",
		Board:    snapshot.Board,
		Symbol:   winner.Symbol(),
		Turn:     snapshot.Turn,
		Status:   "won",
		Text:     "You win.",
		Opponent: loser.Name(),
		Winner:   winnerSymbol,
	})

	_ = loser.Send(wire.Message{
		Type:     "state",
		Board:    snapshot.Board,
		Symbol:   loser.Symbol(),
		Turn:     snapshot.Turn,
		Status:   "lost",
		Text:     fmt.Sprintf("%s wins.", winner.Name()),
		Opponent: winner.Name(),
		Winner:   winnerSymbol,
	})
}

func (s *Session) sendDraw(first, second *Player) {
	snapshot := s.game.Snapshot()
	msg := wire.Message{
		Type:   "state",
		Board:  snapshot.Board,
		Turn:   snapshot.Turn,
		Status: "draw",
		Text:   "Game over. Draw.",
	}

	msg.Symbol = first.Symbol()
	msg.Opponent = second.Name()
	_ = first.Send(msg)

	msg.Symbol = second.Symbol()
	msg.Opponent = first.Name()
	_ = second.Send(msg)
}

func (s *Session) endByDisconnect(winner, leaver *Player) {
	if s.ctx.Err() != nil {
		// During server shutdown we silently stop sessions instead of reporting a fake forfeit.
		return
	}
	_ = winner.Send(wire.Message{
		Type:     "state",
		Status:   "won",
		Text:     fmt.Sprintf("%s disconnected. You win by forfeit.", leaver.Name()),
		Symbol:   winner.Symbol(),
		Opponent: leaver.Name(),
		Winner:   winner.Symbol(),
	})
	s.requeue(winner)
	log.Printf("[session] ended by disconnect: %s left, %s remains", leaver.Name(), winner.Name())
}

func (s *Session) requeue(players ...*Player) {
	if s.ctx.Err() != nil {
		return
	}

	for _, player := range players {
		if player == nil || !player.Alive() {
			continue
		}

		_ = player.Send(wire.Message{
			Type:   "info",
			Status: "waiting",
			Text:   "Waiting for a new opponent...",
		})
		select {
		case <-s.ctx.Done():
			return
		// Requeue keeps client connections alive across matches.
		case s.waiting <- player:
		}
	}
}
