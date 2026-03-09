package main

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/urfave/cli/v3"
)

var versionCmd = &cli.Command{
	Name:  "version",
	Usage: "print the version",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		v := "dev"
		if info, ok := debug.ReadBuildInfo(); ok && v == "dev" {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				v = info.Main.Version
			}
		}
		fmt.Printf("storage-police %s\n", v)
		return nil
	},
}
