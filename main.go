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
		Usage:                 "catch storage frauds",
		Description:           "storage-police is a utility to detect fraudulent storages by writing a determinable sequence onto the device and reading it back to verify actual capacity and data integrity.",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			writeCmd,
			readCmd,
			scrubCmd,
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
