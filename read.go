package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
	"lukechampine.com/blake3"
)

type readConfig struct {
	seed   string
	device string
	info   os.FileInfo
}

type readerApp struct {
	ctx            context.Context
	cfg            *readConfig
	stream         io.Reader
	device         *os.File
	deviceCapacity int
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
	cfg, err := parseReadConfig(cmd)
	if err != nil {
		return nil, err
	}
	return &readerApp{ctx: ctx, cfg: cfg}, nil
}

func (a *readerApp) run() error {
	a.hashSeed()

	if err := a.openDevice(); err != nil {
		return err
	}
	defer a.device.Close()

	if err := a.displayInfo(); err != nil {
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
	firstPrint := true

	history := []progressRecord{{timestamp: startTime, bytes: 0}}

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
			return cli.Exit(fmt.Sprintf("\nVerification failed at offset %d: data mismatch\n", readBytes), 1)
		}

		readBytes += int64(chunkLen)

		// Print progress every 500ms
		if time.Since(lastPrint) > 500*time.Millisecond {
			now := time.Now()
			lastPrint = now

			history = append(history, progressRecord{timestamp: now, bytes: readBytes})
			// keep last 5 seconds of history to display the current read speed
			cutoff := now.Add(-5 * time.Second)
			keepFrom := 0
			maxDiscard := len(history) - 1
			for keepFrom < maxDiscard && history[keepFrom+1].timestamp.Before(cutoff) {
				keepFrom++
			}
			history = history[keepFrom:]

			// calculate displayed stats
			oldest := history[0]
			elapsed := now.Sub(oldest.timestamp).Seconds()
			speed := 0.0
			if elapsed > 0 {
				speed = float64(readBytes-oldest.bytes) / elapsed
			}
			percent := 0.0
			if a.deviceCapacity > 0 {
				percent = float64(readBytes) / float64(a.deviceCapacity) * 100
			}

			// display stats
			t := table.NewWriter()
			t.SetStyle(table.StyleRounded)
			t.SetColumnConfigs([]table.ColumnConfig{
				{Number: 1, AlignHeader: text.AlignCenter, Align: text.AlignRight, WidthMin: 10},
				{Number: 2, AlignHeader: text.AlignCenter, Align: text.AlignRight, WidthMin: 10},
				{Number: 3, AlignHeader: text.AlignCenter, Align: text.AlignRight, WidthMin: 10},
				{Number: 4, AlignHeader: text.AlignCenter, Align: text.AlignRight, WidthMin: 12},
			})
			t.AppendHeader(table.Row{"Progress", "Total", "Percentage", "Speed"})
			t.AppendRow(table.Row{
				humanize.Bytes(uint64(readBytes)),
				humanize.Bytes(uint64(a.deviceCapacity)),
				fmt.Sprintf("%.2f%%", percent),
				humanize.Bytes(uint64(speed)) + "/s",
			})

			output := t.Render()
			numLines := strings.Count(output, "\n") + 1
			if !firstPrint {
				fmt.Printf("\r\033[%dA", numLines)
			}
			fmt.Println(output)
			firstPrint = false
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

func parseReadConfig(cmd *cli.Command) (*readConfig, error) {
	if cmd.Args().Present() {
		return nil, fmt.Errorf("unknown argument: %s", cmd.Args().First())
	}
	seed := cmd.String("seed")
	if seed == "" {
		return nil, fmt.Errorf("seed is required")
	}
	device := cmd.StringArg("device")

	fileInfo, err := os.Stat(device)
	if err != nil {
		return nil, err
	}
	mode := fileInfo.Mode()
	if mode&os.ModeDevice == 0 || mode&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("device %s is not a block device", device)
	}

	return &readConfig{seed: seed, device: device, info: fileInfo}, nil
}

func (a *readerApp) hashSeed() {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte(a.cfg.seed))
	a.stream = hasher.XOF()
}

func (a *readerApp) openDevice() error {
	device := a.cfg.device
	f, err := os.OpenFile(device, os.O_RDONLY|unix.O_DIRECT, 0666)
	if err != nil {
		return err
	}

	size, err := getBlockDeviceSize(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("could not get size of block device %s: %w", device, err)
	}

	a.deviceCapacity = size
	a.device = f
	return nil
}

func (a *readerApp) displayInfo() error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Device", "Size", "Seed"})
	t.AppendRow(table.Row{
		a.cfg.device,
		humanize.Bytes(uint64(a.deviceCapacity)),
		a.cfg.seed,
	})
	t.Render()
	fmt.Println("Starting verification process. This might take a while...")
	return nil
}
