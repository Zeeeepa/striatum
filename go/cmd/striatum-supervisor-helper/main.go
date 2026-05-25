package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/halbritt/striatum/go/pkg/supervisor"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := supervisor.RunHelper(ctx, os.Stdin, os.Stdout, supervisor.HelperOptions{}); err != nil {
		os.Exit(1)
	}
}
