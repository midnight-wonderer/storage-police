package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/charmbracelet/huh"
	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
	"lukechampine.com/blake3"
)

type writeConfig struct {
	seed   string
	device string
	info   os.FileInfo
}

type app struct {
	ctx            context.Context
	cfg            *writeConfig
	stream         io.Reader
	device         *os.File
	deviceCapacity int
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
		app, err := newWriter(ctx, cmd)
		if err != nil {
			return err
		}
		return app.run()
	},
}

func newWriter(ctx context.Context, cmd *cli.Command) (*app, error) {
	cfg, err := parseConfig(cmd)
	if err != nil {
		return nil, err
	}
	return &app{ctx: ctx, cfg: cfg}, nil
}

func (a *app) run() error {
	if err := a.confirm(); err != nil {
		return err
	}

	a.hashSeed()

	if err := a.openDevice(); err != nil {
		return err
	}
	defer a.device.Close()

	if err := a.displayInfo(); err != nil {
		return err
	}

	return a.performWrite()
}

type progressRecord struct {
	timestamp time.Time
	bytes     int64
}

func (a *app) performWrite() error {
	buf := allocateAligned(1024*1024, 4096) // 1 MiB chunk, 4K aligned for O_DIRECT
	written := int64(0)
	startTime := time.Now()
	lastPrint := time.Now()
	firstPrint := true

	history := []progressRecord{{timestamp: startTime, bytes: 0}}

	for {
		select {
		case <-a.ctx.Done():
			return cli.Exit("\nWrite interrupted by user.\n", 1)
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
				fmt.Printf("\nWrite interrupted or failed: %v\n", wErr)
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

			history = append(history, progressRecord{timestamp: now, bytes: written})
			// keep last 5 seconds of history to display the current write speed
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
				speed = float64(written-oldest.bytes) / elapsed
			}
			percent := 0.0
			if a.deviceCapacity > 0 {
				percent = float64(written) / float64(a.deviceCapacity) * 100
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
				humanize.Bytes(uint64(written)),
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
	averageSpeed := float64(written) / timeTaken.Seconds()

	fmt.Printf(
		"\nWrite successful.\nTime taken: %s, Average write speed: %s/s\n",
		timeTaken,
		humanize.Bytes(uint64(averageSpeed)),
	)

	return a.device.Sync()
}

func parseConfig(cmd *cli.Command) (*writeConfig, error) {
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

	return &writeConfig{seed: seed, device: device, info: fileInfo}, nil
}

func (a *app) confirm() error {
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

func (a *app) hashSeed() {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte(a.cfg.seed))
	a.stream = hasher.XOF()
}

func (a *app) openDevice() error {
	device := a.cfg.device
	f, err := os.OpenFile(device, os.O_WRONLY|unix.O_DIRECT, 0666)
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

func (a *app) displayInfo() error {
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
	fmt.Println("Starting write process. This might take a while...")
	return nil
}

func getBlockDeviceSize(f *os.File) (int, error) {
	size, err := unix.IoctlGetInt(int(f.Fd()), unix.BLKGETSIZE64)
	if err != nil {
		return 0, fmt.Errorf("could not get size of block device: %w", err)
	}
	return size, nil
}

func allocateAligned(size, align int) []byte {
	buf := make([]byte, size+align-1)
	offset := int(uintptr(unsafe.Pointer(&buf[0])) % uintptr(align))
	if offset != 0 {
		offset = align - offset
	}
	return buf[offset : offset+size]
}
