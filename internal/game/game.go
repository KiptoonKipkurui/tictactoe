package game

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidSymbol = errors.New("invalid symbol")
	ErrOutOfBounds   = errors.New("move is out of bounds")
	ErrCellOccupied  = errors.New("cell is already occupied")
	ErrWrongTurn     = errors.New("it is not this symbol's turn")
)

type Game struct {
	board [9]string
	turn  string
}

// New creates a fresh tic-tac-toe game where X always moves first.
func New() *Game {
	return &Game{turn: "X"}
}

func (g *Game) Board() [9]string {
	return g.board
}

func (g *Game) Turn() string {
	return g.turn
}

func (g *Game) Snapshot() Snapshot {
	return Snapshot{
		Board: g.board,
		Turn:  g.turn,
	}
}

func (g *Game) ApplyMove(row, col int, symbol string) error {
	return g.MakeMove(row, col, symbol)
}

func (g *Game) MakeMove(row, col int, symbol string) error {
	if symbol != "X" && symbol != "O" {
		return ErrInvalidSymbol
	}
	if row < 0 || row > 2 || col < 0 || col > 2 {
		return ErrOutOfBounds
	}
	if g.turn != symbol {
		return ErrWrongTurn
	}

	index := row*3 + col
	if g.board[index] != "" {
		return fmt.Errorf("%w: row=%d col=%d", ErrCellOccupied, row+1, col+1)
	}

	g.board[index] = symbol
	if symbol == "X" {
		g.turn = "O"
	} else {
		g.turn = "X"
	}

	return nil
}

func (g *Game) Winner() string {
	// These are the 8 winning lines in a 3x3 board: rows, columns, and diagonals.
	lines := [8][3]int{
		{0, 1, 2},
		{3, 4, 5},
		{6, 7, 8},
		{0, 3, 6},
		{1, 4, 7},
		{2, 5, 8},
		{0, 4, 8},
		{2, 4, 6},
	}

	for _, line := range lines {
		a, b, c := line[0], line[1], line[2]
		if g.board[a] != "" && g.board[a] == g.board[b] && g.board[b] == g.board[c] {
			return g.board[a]
		}
	}

	return ""
}

func (g *Game) Draw() bool {
	if g.Winner() != "" {
		return false
	}
	for _, cell := range g.board {
		if cell == "" {
			return false
		}
	}
	return true
}
