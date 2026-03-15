package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/kiptoon/tictactoe/internal/server"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the tic-tac-toe server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return server.New(addr).Run(ctx)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":3333", "TCP listen address")

	return cmd
}
