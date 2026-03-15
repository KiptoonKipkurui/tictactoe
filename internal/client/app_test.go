package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/kiptoon/tictactoe/internal/wire"
)

type stubUI struct {
	mu     sync.Mutex
	infos  []string
	errors []string
	states []wire.Message
}

func (u *stubUI) SetController(Controller) {}

func (u *stubUI) Run(context.Context) error { return nil }

func (u *stubUI) ShowInfo(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.infos = append(u.infos, text)
}

func (u *stubUI) ShowError(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.errors = append(u.errors, text)
}

func (u *stubUI) RenderState(msg wire.Message) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.states = append(u.states, msg)
}

func TestSubmitMoveClearsLocalTurnUntilServerResponds(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	app := New(nil, &stubUI{})
	app.conn = clientConn
	app.peer = wire.NewJSONConn(clientConn)
	app.current = wire.Message{
		Status: "playing",
		Symbol: "X",
		Turn:   "X",
	}

	msgs := make(chan wire.Message, 1)
	go func() {
		defer close(msgs)
		var msg wire.Message
		if err := json.NewDecoder(serverConn).Decode(&msg); err == nil {
			msgs <- msg
		}
	}()

	if err := app.SubmitMove(1, 2); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	msg := <-msgs
	if msg.Type != "move" || msg.Row != 1 || msg.Col != 2 {
		t.Fatalf("sent message = %+v, want move row=1 col=2", msg)
	}

	if got := app.current.Turn; got != "" {
		t.Fatalf("current.Turn = %q, want empty until server response", got)
	}
}

func TestReadServerLoopRestoresTurnOnError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ui := &stubUI{}
	app := New(nil, ui)
	app.current = wire.Message{
		Status: "playing",
		Symbol: "X",
		Turn:   "",
	}

	errs := make(chan error, 1)
	go func() {
		errs <- app.readServerLoop(clientConn)
	}()

	encoder := json.NewEncoder(serverConn)
	if err := encoder.Encode(wire.Message{Type: "error", Text: "cell is already occupied"}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	_ = serverConn.Close()

	if err := <-errs; err != nil && err != io.EOF {
		t.Fatalf("readServerLoop() error = %v, want nil or EOF", err)
	}

	if got := app.current.Turn; got != "X" {
		t.Fatalf("current.Turn = %q, want restored player symbol", got)
	}

	if len(ui.errors) != 1 || ui.errors[0] != "cell is already occupied" {
		t.Fatalf("ui errors = %#v, want single server error", ui.errors)
	}
}

func TestSubmitMoveRequiresActiveTurn(t *testing.T) {
	app := New(nil, &stubUI{})
	app.current = wire.Message{
		Status: "playing",
		Symbol: "X",
		Turn:   "O",
	}

	if err := app.SubmitMove(0, 0); err != ErrNotYourTurn {
		t.Fatalf("SubmitMove() error = %v, want %v", err, ErrNotYourTurn)
	}
}
