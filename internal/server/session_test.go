package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/kiptoon/tictactoe/internal/game"
	"github.com/kiptoon/tictactoe/internal/wire"
)

func TestSessionHandleMessageOccupiedCellSendsErrorOnlyToActor(t *testing.T) {
	session, xPlayer, xMsgs, oPlayer, oMsgs, waiting := newTestSession(t)

	mustApplyMove(t, session.game, 0, 0, "X")
	mustApplyMove(t, session.game, 1, 1, "O")

	keepRunning := session.handleMessage(xPlayer, oPlayer, wire.Message{
		Type: "move",
		Row:  0,
		Col:  0,
	})

	if !keepRunning {
		t.Fatal("handleMessage() = false, want true for invalid move")
	}

	msg := mustReadMessage(t, xMsgs)
	if msg.Type != "error" {
		t.Fatalf("actor message type = %q, want error", msg.Type)
	}

	assertNoMessage(t, oMsgs)
	assertNoWaitingPlayer(t, waiting)
}

func TestSessionHandleMessageWinSendsResultsAndRequeuesBothPlayers(t *testing.T) {
	session, xPlayer, xMsgs, oPlayer, oMsgs, waiting := newTestSession(t)

	mustApplyMove(t, session.game, 0, 0, "X")
	mustApplyMove(t, session.game, 1, 0, "O")
	mustApplyMove(t, session.game, 0, 1, "X")
	mustApplyMove(t, session.game, 1, 1, "O")

	keepRunning := session.handleMessage(xPlayer, oPlayer, wire.Message{
		Type: "move",
		Row:  0,
		Col:  2,
	})

	if keepRunning {
		t.Fatal("handleMessage() = true, want false after win")
	}

	xResult := mustReadMessage(t, xMsgs)
	if xResult.Status != "won" || xResult.Winner != "X" {
		t.Fatalf("winner message = %+v, want won by X", xResult)
	}

	oResult := mustReadMessage(t, oMsgs)
	if oResult.Status != "lost" || oResult.Winner != "X" {
		t.Fatalf("loser message = %+v, want lost by X", oResult)
	}

	xInfo := mustReadMessage(t, xMsgs)
	if xInfo.Type != "info" || xInfo.Status != "waiting" {
		t.Fatalf("winner requeue message = %+v, want waiting info", xInfo)
	}

	oInfo := mustReadMessage(t, oMsgs)
	if oInfo.Type != "info" || oInfo.Status != "waiting" {
		t.Fatalf("loser requeue message = %+v, want waiting info", oInfo)
	}

	firstQueued := mustReadWaitingPlayer(t, waiting)
	secondQueued := mustReadWaitingPlayer(t, waiting)
	if (firstQueued != xPlayer && firstQueued != oPlayer) || (secondQueued != xPlayer && secondQueued != oPlayer) || firstQueued == secondQueued {
		t.Fatalf("queued players = %p, %p; want both session players", firstQueued, secondQueued)
	}
}

func TestSessionEndByDisconnectAwardsWinnerAndRequeues(t *testing.T) {
	session, _, _, oPlayer, oMsgs, waiting := newTestSession(t)
	leaver, leaverReadEnd := newTestPlayer(t, "leaver", "X")
	defer leaverReadEnd.Close()

	session.endByDisconnect(oPlayer, leaver)

	result := mustReadMessage(t, oMsgs)
	if result.Status != "won" || result.Winner != "O" {
		t.Fatalf("disconnect result = %+v, want won by O", result)
	}

	info := mustReadMessage(t, oMsgs)
	if info.Type != "info" || info.Status != "waiting" {
		t.Fatalf("disconnect requeue message = %+v, want waiting info", info)
	}

	queued := mustReadWaitingPlayer(t, waiting)
	if queued != oPlayer {
		t.Fatalf("queued player = %p, want %p", queued, oPlayer)
	}
}

func newTestSession(t *testing.T) (*Session, *Player, <-chan wire.Message, *Player, <-chan wire.Message, chan *Player) {
	t.Helper()

	xPlayer, xReadEnd := newTestPlayer(t, "alice", "X")
	t.Cleanup(func() { _ = xReadEnd.Close() })

	oPlayer, oReadEnd := newTestPlayer(t, "bob", "O")
	t.Cleanup(func() { _ = oReadEnd.Close() })

	waiting := make(chan *Player, 4)
	session := &Session{
		ctx:     context.Background(),
		game:    game.New(),
		waiting: waiting,
		xPlayer: xPlayer,
		oPlayer: oPlayer,
	}

	return session, xPlayer, decodeMessages(t, xReadEnd), oPlayer, decodeMessages(t, oReadEnd), waiting
}

func newTestPlayer(t *testing.T, name, symbol string) (*Player, net.Conn) {
	t.Helper()

	writer, reader := net.Pipe()
	player := NewPlayer(name, writer)
	player.SetSymbol(symbol)
	return player, reader
}

func decodeMessages(t *testing.T, conn net.Conn) <-chan wire.Message {
	t.Helper()

	out := make(chan wire.Message, 8)
	go func() {
		defer close(out)
		decoder := json.NewDecoder(conn)
		for {
			var msg wire.Message
			if err := decoder.Decode(&msg); err != nil {
				return
			}
			out <- msg
		}
	}()

	return out
}

func mustApplyMove(t *testing.T, g game.Engine, row, col int, symbol string) {
	t.Helper()

	if err := g.ApplyMove(row, col, symbol); err != nil {
		t.Fatalf("ApplyMove(%d, %d, %q) error = %v", row, col, symbol, err)
	}
}

func mustReadMessage(t *testing.T, msgs <-chan wire.Message) wire.Message {
	t.Helper()

	select {
	case msg := <-msgs:
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
		return wire.Message{}
	}
}

func assertNoMessage(t *testing.T, msgs <-chan wire.Message) {
	t.Helper()

	select {
	case msg := <-msgs:
		t.Fatalf("unexpected message: %+v", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

func mustReadWaitingPlayer(t *testing.T, waiting <-chan *Player) *Player {
	t.Helper()

	select {
	case player := <-waiting:
		return player
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued player")
		return nil
	}
}

func assertNoWaitingPlayer(t *testing.T, waiting <-chan *Player) {
	t.Helper()

	select {
	case player := <-waiting:
		t.Fatalf("unexpected queued player: %v", player.Name())
	case <-time.After(200 * time.Millisecond):
	}
}
