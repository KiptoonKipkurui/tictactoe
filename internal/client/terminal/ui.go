package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/kiptoon/tictactoe/internal/client"
	"github.com/kiptoon/tictactoe/internal/wire"
)

type UI struct {
	in  io.Reader
	out io.Writer
	err io.Writer

	mu sync.Mutex
	// controller lets the terminal delegate actions to the client runtime.
	controller client.Controller
	// canMove mirrors the latest server state so the terminal can suppress invalid input locally.
	canMove bool
}

// NewUI creates the terminal implementation of the client UI contract.
func NewUI(in io.Reader, out, err io.Writer) *UI {
	return &UI{
		in:  in,
		out: out,
		err: err,
	}
}

func (u *UI) SetController(controller client.Controller) {
	u.controller = controller
}

// Run is a simple readline loop that translates terminal input into controller actions.
func (u *UI) Run(ctx context.Context) error {
	u.printHelp()

	inputLines := make(chan string, 16)
	go u.readInput(inputLines)

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-inputLines:
			if !ok {
				return nil
			}

			line = strings.TrimSpace(line)
			if line == "" {
				u.printPrompt()
				continue
			}

			if strings.EqualFold(line, "quit") || strings.EqualFold(line, "exit") {
				fmt.Fprintln(u.out, "Goodbye.")
				return u.controller.Quit()
			}

			// In the terminal we cannot literally disable stdin, so we ignore move input until allowed.
			if !u.canSubmitMove() {
				continue
			}

			row, col, err := parseMove(line)
			if err != nil {
				fmt.Fprintf(u.err, "Invalid move: %v\n", err)
				u.printPrompt()
				continue
			}

			if err := u.controller.SubmitMove(row, col); err != nil {
				if err == client.ErrNotYourTurn {
					fmt.Fprintln(u.err, "It is not your turn yet.")
				} else {
					fmt.Fprintf(u.err, "Send move failed: %v\n", err)
				}
				u.printPrompt()
			}
		}
	}
}

func (u *UI) ShowInfo(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	fmt.Fprintln(u.out, text)
}

func (u *UI) ShowError(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	fmt.Fprintf(u.err, "Server error: %s\n", text)
	if u.canMove {
		u.printPromptLocked()
	}
}

// RenderState redraws the current match snapshot and refreshes the local move gate.
func (u *UI) RenderState(msg wire.Message) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// The terminal should only accept move input when the server says it is our turn.
	u.canMove = msg.Status == "playing" && msg.Turn == msg.Symbol

	if msg.Opponent != "" {
		fmt.Fprintf(u.out, "Opponent: %s\n", msg.Opponent)
	}
	if msg.Symbol != "" {
		fmt.Fprintf(u.out, "You are: %s\n", msg.Symbol)
	}
	fmt.Fprintln(u.out, formatBoard(msg.Board))
	if msg.Text != "" {
		fmt.Fprintln(u.out, msg.Text)
	}
	if u.canMove {
		u.printPromptLocked()
	}
}

func (u *UI) canSubmitMove() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.canMove
}

// readInput keeps stdin handling isolated from the render path.
func (u *UI) readInput(out chan<- string) {
	defer close(out)

	scanner := bufio.NewScanner(u.in)
	for scanner.Scan() {
		out <- scanner.Text()
	}
}

func (u *UI) printHelp() {
	u.mu.Lock()
	defer u.mu.Unlock()

	fmt.Fprintln(u.out, "Commands:")
	fmt.Fprintln(u.out, "  1-9   place a mark into the numbered cell")
	fmt.Fprintln(u.out, "  r c   place a mark using row and column, both 1-3")
	fmt.Fprintln(u.out, "  quit  leave the game")
}

func (u *UI) printPrompt() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.printPromptLocked()
}

func (u *UI) printPromptLocked() {
	fmt.Fprint(u.out, "Your move: ")
}

func formatBoard(board [9]string) string {
	cells := make([]string, len(board))
	for i, cell := range board {
		if cell == "" {
			cells[i] = strconv.Itoa(i + 1)
		} else {
			cells[i] = cell
		}
	}

	return fmt.Sprintf(
		" %s | %s | %s\n---+---+---\n %s | %s | %s\n---+---+---\n %s | %s | %s",
		cells[0], cells[1], cells[2],
		cells[3], cells[4], cells[5],
		cells[6], cells[7], cells[8],
	)
}

// parseMove accepts either a board cell number (1-9) or a row/column pair (1-3 1-3).
func parseMove(input string) (int, int, error) {
	fields := strings.Fields(input)
	switch len(fields) {
	case 1:
		cell, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, 0, fmt.Errorf("expected cell number 1-9 or row col")
		}
		if cell < 1 || cell > 9 {
			return 0, 0, fmt.Errorf("cell must be between 1 and 9")
		}
		cell--
		return cell / 3, cell % 3, nil
	case 2:
		row, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, 0, fmt.Errorf("row must be a number")
		}
		col, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, 0, fmt.Errorf("col must be a number")
		}
		if row < 1 || row > 3 || col < 1 || col > 3 {
			return 0, 0, fmt.Errorf("row and col must be between 1 and 3")
		}
		return row - 1, col - 1, nil
	default:
		return 0, 0, fmt.Errorf("use either `5` or `2 3`")
	}
}
