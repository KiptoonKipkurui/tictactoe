package game

import "testing"

func TestWinner(t *testing.T) {
	g := New()
	moves := []struct {
		row, col int
		symbol   string
	}{
		{0, 0, "X"},
		{1, 0, "O"},
		{0, 1, "X"},
		{1, 1, "O"},
		{0, 2, "X"},
	}

	for _, move := range moves {
		if err := g.MakeMove(move.row, move.col, move.symbol); err != nil {
			t.Fatalf("MakeMove() error = %v", err)
		}
	}

	if got := g.Winner(); got != "X" {
		t.Fatalf("Winner() = %q, want %q", got, "X")
	}
}

func TestDraw(t *testing.T) {
	g := New()
	moves := []struct {
		row, col int
		symbol   string
	}{
		{0, 0, "X"},
		{0, 1, "O"},
		{0, 2, "X"},
		{1, 1, "O"},
		{1, 0, "X"},
		{1, 2, "O"},
		{2, 1, "X"},
		{2, 0, "O"},
		{2, 2, "X"},
	}

	for _, move := range moves {
		if err := g.MakeMove(move.row, move.col, move.symbol); err != nil {
			t.Fatalf("MakeMove() error = %v", err)
		}
	}

	if !g.Draw() {
		t.Fatal("Draw() = false, want true")
	}
}
