package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiptoon/tictactoe/internal/client"
	"github.com/kiptoon/tictactoe/internal/client/terminal"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var addr string
	var name string
	var retryDelay time.Duration

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Run the tic-tac-toe client",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClient(addr, name, retryDelay)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:3333", "Server address")
	cmd.Flags().StringVar(&name, "name", "", "Player name")
	cmd.Flags().DurationVar(&retryDelay, "retry-delay", 2*time.Second, "Initial delay between connection retries")

	return cmd
}

func runClient(addr, name string, retryDelay time.Duration) error {
	ui := terminal.NewUI(os.Stdin, os.Stdout, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return client.RunWithUI(ctx, ui, client.Options{
		Addr:       addr,
		Name:       name,
		RetryDelay: retryDelay,
	})
}
