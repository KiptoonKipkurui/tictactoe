package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type Options struct {
	// Addr is the TCP address of the server, for example 127.0.0.1:3333.
	Addr string
	// Name is the preferred player name. Blank means "derive one locally".
	Name string
	// RetryDelay is the initial reconnect delay before exponential backoff kicks in.
	RetryDelay time.Duration
}

// RunWithUI wires the reconnecting client runtime to the supplied UI implementation.
func RunWithUI(ctx context.Context, ui UI, opts Options) error {
	playerName := resolvePlayerName(opts.Name)
	app := New(func(ctx context.Context) (net.Conn, error) {
		return connectWithRetry(ctx, opts.Addr, opts.RetryDelay)
	}, ui)

	if err := app.Run(ctx, playerName); err != nil {
		return fmt.Errorf("client run: %w", err)
	}

	return nil
}

func resolvePlayerName(name string) string {
	playerName := strings.TrimSpace(name)
	if playerName != "" {
		return playerName
	}

	// Use the hostname as a lightweight default identity across reconnects.
	host, err := os.Hostname()
	if err != nil {
		return "Anonymous"
	}

	return host
}

func connectWithRetry(ctx context.Context, addr string, retryDelay time.Duration) (net.Conn, error) {
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}

	delay := retryDelay
	const maxDelay = 30 * time.Second

	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn, nil
		}

		// The client stays alive when the server is unavailable and retries with capped backoff.
		fmt.Fprintf(os.Stderr, "Server at %s is unavailable (%v). Retrying in %s...\n", addr, err, delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}
