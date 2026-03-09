package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v3"
)

type readerApp struct {
	baseApp
}

var readCmd = &cli.Command{
	Name: "read",
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
	Usage: "verify a pseudorandom binary sequence from a drive",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		app, err := newReader(ctx, cmd)
		if err != nil {
			return err
		}
		return app.run()
	},
}

func newReader(ctx context.Context, cmd *cli.Command) (*readerApp, error) {
	cfg, err := parseDeviceConfig(cmd)
	if err != nil {
		return nil, err
	}
	return &readerApp{baseApp: baseApp{ctx: ctx, cfg: cfg}}, nil
}

func (a *readerApp) run() error {
	a.hashSeed()

	if err := a.openDevice(os.O_RDONLY); err != nil {
		return err
	}
	defer a.device.Close()

	if err := a.displayInfo("verification"); err != nil {
		return err
	}

	return a.performRead()
}

func (a *readerApp) performRead() error {
	buf := allocateAligned(1024*1024, 4096)
	expectedBuf := allocateAligned(1024*1024, 4096)
	readBytes := int64(0)
	startTime := time.Now()
	lastPrint := time.Now()

	pt := newProgressTracker(startTime, a.deviceCapacity)

	for {
		select {
		case <-a.ctx.Done():
			return cli.Exit("\nVerification interrupted by user.\n", 1)
		default:
		}

		chunkLen, rErr := a.device.Read(buf)
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			fmt.Printf("\nRead interrupted or failed: %v\n", rErr)
			return rErr
		}

		if chunkLen <= 0 {
			continue
		}

		_, err := io.ReadFull(a.stream, expectedBuf[:chunkLen])
		if err != nil {
			return err
		}

		if !bytes.Equal(buf[:chunkLen], expectedBuf[:chunkLen]) {
			if readBytes < 16 {
				return cli.Exit("\nVerification failed. Please check your seed.\n", 1)
			}
			return cli.Exit(fmt.Sprintf("\nVerification failed at offset %d: data mismatch\n", readBytes), 1)
		}

		readBytes += int64(chunkLen)

		// Print progress every 500ms
		if time.Since(lastPrint) > 500*time.Millisecond {
			now := time.Now()
			lastPrint = now
			pt.print(now, readBytes)
		}
	}

	timeTaken := time.Since(startTime).Round(time.Millisecond)
	averageSpeed := float64(readBytes) / timeTaken.Seconds()

	fmt.Printf(
		"\nVerification successful.\nTime taken: %s, Average read speed: %s/s\n",
		timeTaken,
		humanize.Bytes(uint64(averageSpeed)),
	)

	return nil
}
