package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/kiptoon/tictactoe/internal/wire"
)

var ErrNotYourTurn = errors.New("it is not your turn")
var ErrDisconnected = errors.New("not connected to the server")

type DialFunc func(ctx context.Context) (net.Conn, error)

// Controller is the action surface the UI can invoke without knowing transport details.
type Controller interface {
	SubmitMove(row, col int) error
	Quit() error
}

// UI is the presentation contract used by the client runtime.
type UI interface {
	SetController(Controller)
	Run(ctx context.Context) error
	ShowInfo(text string)
	ShowError(text string)
	RenderState(msg wire.Message)
}

// App owns the client-side connection lifecycle and delegates presentation to a UI implementation.
type App struct {
	// dial is called whenever the app needs a fresh server connection.
	dial DialFunc
	// ui renders state and collects user input.
	ui UI

	mu sync.RWMutex
	// conn and peer represent the current live server connection, if any.
	conn net.Conn
	peer wire.Conn
	// current is the latest authoritative state received from the server.
	current wire.Message
	// shuttingDown prevents reconnect logic from spinning back up during exit.
	shuttingDown bool
}

func New(dial DialFunc, ui UI) *App {
	app := &App{
		dial: dial,
		ui:   ui,
	}
	ui.SetController(app)
	return app
}

// Run keeps the UI alive across reconnects and re-establishes the server session when the connection drops.
func (a *App) Run(ctx context.Context, name string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	uiErrs := make(chan error, 1)
	go func() {
		uiErrs <- a.ui.Run(ctx)
	}()

	for {
		select {
		case err := <-uiErrs:
			a.markShuttingDown()
			a.closeCurrentConnection()
			cancel()
			if err != nil {
				return fmt.Errorf("run ui: %w", err)
			}
			return nil
		case <-ctx.Done():
			a.markShuttingDown()
			a.closeCurrentConnection()
			return nil
		default:
		}

		conn, err := a.dial(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("dial server: %w", err)
		}

		if err := a.attach(conn, name); err != nil {
			_ = conn.Close()
			if ctx.Err() != nil {
				return nil
			}
			a.ui.ShowError(fmt.Sprintf("Reconnect failed: %v", err))
			continue
		}

		a.ui.ShowInfo("Connected to server.")

		serverErrs := make(chan error, 1)
		go func(activeConn net.Conn) {
			serverErrs <- a.readServerLoop(activeConn)
		}(conn)

		select {
		case err := <-uiErrs:
			a.markShuttingDown()
			a.closeCurrentConnection()
			cancel()
			if err != nil {
				return fmt.Errorf("run ui: %w", err)
			}
			return nil
		case err := <-serverErrs:
			// The read loop is tied to one connection instance, so drop it before reconnecting.
			a.detach(conn)
			if a.isShuttingDown() || ctx.Err() != nil {
				return nil
			}
			if err != nil && !errors.Is(err, io.EOF) {
				a.ui.ShowError(fmt.Sprintf("Connection lost: %v", err))
			} else {
				a.ui.ShowInfo("Connection lost. Reconnecting...")
			}
		case <-ctx.Done():
			a.markShuttingDown()
			a.closeCurrentConnection()
			return nil
		}
	}
}

// SubmitMove validates the local turn snapshot before sending a move to the server.
func (a *App) SubmitMove(row, col int) error {
	a.mu.RLock()
	current := a.current
	peer := a.peer
	a.mu.RUnlock()

	if current.Status != "playing" || current.Turn != current.Symbol {
		return ErrNotYourTurn
	}
	if peer == nil {
		return ErrDisconnected
	}

	if err := peer.Send(wire.Message{
		Type: "move",
		Row:  row,
		Col:  col,
	}); err != nil {
		return err
	}

	// Prevent duplicate local submissions until the server confirms the next state.
	a.mu.Lock()
	if a.current.Symbol == current.Symbol && a.current.Turn == current.Symbol {
		a.current.Turn = ""
	}
	a.mu.Unlock()

	return nil
}

func (a *App) Quit() error {
	a.markShuttingDown()
	return a.closeCurrentConnection()
}

// attach performs the hello handshake and makes the connection current for the app.
func (a *App) attach(conn net.Conn, name string) error {
	peer := wire.NewJSONConn(conn)
	if err := peer.Send(wire.Message{
		Type: "hello",
		Name: name,
	}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	a.mu.Lock()
	a.conn = conn
	a.peer = peer
	a.current = wire.Message{}
	a.mu.Unlock()

	return nil
}

func (a *App) detach(conn net.Conn) {
	a.mu.Lock()
	if a.conn == conn {
		a.conn = nil
		a.peer = nil
		a.current = wire.Message{}
	}
	a.mu.Unlock()
	_ = conn.Close()
}

// closeCurrentConnection clears the current connection state before closing the socket.
func (a *App) closeCurrentConnection() error {
	a.mu.Lock()
	conn := a.conn
	a.conn = nil
	a.peer = nil
	a.current = wire.Message{}
	a.mu.Unlock()

	if conn != nil {
		return conn.Close()
	}

	return nil
}

func (a *App) markShuttingDown() {
	a.mu.Lock()
	a.shuttingDown = true
	a.mu.Unlock()
}

func (a *App) isShuttingDown() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.shuttingDown
}

// readServerLoop processes one live connection until it is closed or decoding fails.
func (a *App) readServerLoop(conn net.Conn) error {
	decoder := json.NewDecoder(conn)
	for {
		var msg wire.Message
		if err := decoder.Decode(&msg); err != nil {
			return err
		}

		switch msg.Type {
		case "info":
			a.ui.ShowInfo(msg.Text)
		case "error":
			// A rejected move still belongs to the same player, so restore the local turn gate.
			a.mu.Lock()
			if a.current.Status == "playing" && a.current.Symbol != "" {
				a.current.Turn = a.current.Symbol
			}
			a.mu.Unlock()
			a.ui.ShowError(msg.Text)
		case "state":
			a.mu.Lock()
			a.current = msg
			a.mu.Unlock()
			a.ui.RenderState(msg)
		default:
			a.ui.ShowError(fmt.Sprintf("Unhandled server message: %+v", msg))
		}
	}
}
