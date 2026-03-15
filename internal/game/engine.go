package game

// Snapshot is the server-facing view of the current game state.
type Snapshot struct {
	Board [9]string
	Turn  string
}

// Engine lets the server run a game session without depending on a specific game implementation.
type Engine interface {
	Snapshot() Snapshot
	ApplyMove(row, col int, symbol string) error
	Winner() string
	Draw() bool
}
