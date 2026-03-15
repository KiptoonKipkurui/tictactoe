package server

import (
	"context"
	"sync"
)

type Matchmaker struct {
	// waiting is the shared lobby queue populated by the server.
	waiting chan *Player
	// queue preserves connection order for fair pairing.
	queue []*Player
	wg    *sync.WaitGroup
}

// NewMatchmaker pairs players in connection order while skipping any stale disconnected entries.
func NewMatchmaker(waiting chan *Player, wg *sync.WaitGroup) *Matchmaker {
	return &Matchmaker{
		waiting: waiting,
		queue:   make([]*Player, 0),
		wg:      wg,
	}
}

func (m *Matchmaker) Run(ctx context.Context) {
	for {
		if len(m.queue) < 2 {
			select {
			case <-ctx.Done():
				return
			case player := <-m.waiting:
				if player.Alive() {
					m.queue = append(m.queue, player)
				}
			}
			continue
		}

		xPlayer := m.popAlive()
		oPlayer := m.popAlive()
		if xPlayer == nil || oPlayer == nil {
			continue
		}

		// Each matched pair gets its own isolated session goroutine.
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			NewSession(ctx, m.waiting, xPlayer, oPlayer).Run()
		}()
	}
}

func (m *Matchmaker) popAlive() *Player {
	for len(m.queue) > 0 {
		player := m.queue[0]
		m.queue = m.queue[1:]
		// Disconnected players may still be sitting in the queue, so skip them lazily here.
		if player.Alive() {
			return player
		}
	}

	return nil
}
