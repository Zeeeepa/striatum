package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/halbritt/striatum/go/pkg/supervisor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := supervisor.RunHelper(ctx, os.Stdin, os.Stdout, supervisor.HelperOptions{}); err != nil {
		os.Exit(1)
	}
}
