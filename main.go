package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := &cli.Command{
		Name:                  "storage-police",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			readCmd,
			writeCmd,
			versionCmd,
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		if s := err.Error(); s != "" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", s)
		}
		if exitErr, ok := err.(cli.ExitCoder); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
