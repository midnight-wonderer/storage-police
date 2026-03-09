package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

type mode int

const (
	modeWrite mode = iota + 1
	modeScrub
)

type writerApp struct {
	baseApp
	mode       mode
	actionWord string
}

var writeCmd = &cli.Command{
	Name: "write",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "seed",
			Usage: "seed for the random number generator",
		},
	},
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name: "device",
		},
	},
	Usage: "write a pseudorandom binary sequence to a drive",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		app, err := newWriter(ctx, cmd, modeWrite, "Write")
		if err != nil {
			return err
		}
		return app.run()
	},
}

var scrubCmd = &cli.Command{
	Name: "scrub",
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name: "device",
		},
	},
	Usage: "securely scrub a drive with a pseudorandom binary sequence",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		app, err := newWriter(ctx, cmd, modeScrub, "Scrub")
		if err != nil {
			return err
		}
		return app.run()
	},
}

func newWriter(ctx context.Context, cmd *cli.Command, m mode, actionWord string) (*writerApp, error) {
	var seed *string
	if m == modeScrub {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("Failed to generate random seed: %w", err)
		}
		s := hex.EncodeToString(b)
		seed = &s
	}

	cfg, err := parseDeviceConfig(cmd, seed)
	if err != nil {
		return nil, err
	}
	a := &writerApp{mode: m, actionWord: actionWord}
	a.ctx = ctx
	a.cfg = cfg
	return a, nil
}

func (a *writerApp) run() error {
	if err := a.confirm(); err != nil {
		return err
	}

	a.hashSeed()

	if err := a.openDevice(os.O_WRONLY); err != nil {
		return err
	}
	defer a.device.Close()

	if err := a.displayInfo(a.actionWord); err != nil {
		return err
	}

	return a.performWrite()
}

func (a *writerApp) performWrite() error {
	buf := allocateAligned(1024*1024, 4096) // 1 MiB chunk, 4K aligned for O_DIRECT
	written := int64(0)
	startTime := time.Now()
	lastPrint := time.Now()

	pt := newProgressTracker(startTime, a.deviceCapacity)

	for {
		select {
		case <-a.ctx.Done():
			return cli.Exit(fmt.Sprintf("\n%s interrupted by user.\n", a.actionWord), 1)
		default:
		}

		// xof implements io.Reader, generating an infinite stream
		chunkLen, err := a.stream.Read(buf)
		if err != nil {
			// since stream is infinite, we should never get an error
			return err
		}

		if chunkLen <= 0 {
			continue
		}

		byteWritten, wErr := a.device.Write(buf[:chunkLen])
		if wErr != nil {
			if !errors.Is(wErr, unix.ENOSPC) {
				fmt.Printf("\n%s interrupted or failed: %v\n", a.actionWord, wErr)
				return wErr
			}
			break // ENOSPC (No space left on device) means we are done
		}

		if byteWritten <= 0 {
			continue
		}
		written += int64(byteWritten)

		// Print progress every 500ms
		if time.Since(lastPrint) > 500*time.Millisecond {
			now := time.Now()
			lastPrint = now
			pt.print(now, written)
		}
	}

	timeTaken := time.Since(startTime).Round(time.Millisecond)
	averageSpeed := float64(written) / timeTaken.Seconds()

	fmt.Printf(
		"\n%s successful.\nTime taken: %s, Average write speed: %s/s\n",
		a.actionWord,
		timeTaken,
		humanize.Bytes(uint64(averageSpeed)),
	)

	return a.device.Sync()
}

func (a *writerApp) confirm() error {
	var confirm bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("WARNING: This will wipe the drive %s.", a.cfg.device)).
		Affirmative("Yes, wipe it").
		Negative("Wait, no!").
		Value(&confirm).
		Run()

	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("operation cancelled")
	}

	return nil
}
