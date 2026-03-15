package server

import (
	"sync"
	"testing"
)

func TestMatchmakerPopAliveSkipsDisconnectedPlayers(t *testing.T) {
	waiting := make(chan *Player, 1)
	matchmaker := NewMatchmaker(waiting, &sync.WaitGroup{})

	dead, deadReadEnd := newTestPlayer(t, "dead", "X")
	defer deadReadEnd.Close()
	dead.Close()

	alive, aliveReadEnd := newTestPlayer(t, "alive", "O")
	defer aliveReadEnd.Close()

	matchmaker.queue = []*Player{dead, alive}

	got := matchmaker.popAlive()
	if got != alive {
		t.Fatalf("popAlive() = %p, want %p", got, alive)
	}
	if len(matchmaker.queue) != 0 {
		t.Fatalf("queue length = %d, want 0", len(matchmaker.queue))
	}
}
