package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/ncw/directio"
	"github.com/urfave/cli/v3"
	"lukechampine.com/blake3"
)

type deviceConfig struct {
	seed          string
	device        string
	info          os.FileInfo
	invertPattern bool
}

type baseApp struct {
	ctx            context.Context
	cfg            *deviceConfig
	stream         io.Reader
	device         *os.File
	deviceCapacity int
}

type progressRecord struct {
	timestamp time.Time
	bytes     int64
}

func parseDeviceConfig(cmd *cli.Command, seedParam *string) (*deviceConfig, error) {
	if cmd.Args().Present() {
		return nil, fmt.Errorf("unknown argument: %s", cmd.Args().First())
	}

	var seed string
	if seedParam != nil {
		seed = *seedParam
	} else {
		seed = cmd.String("seed")
	}
	if seed == "" {
		return nil, fmt.Errorf("seed is required")
	}
	if len(seed) > 64 {
		return nil, fmt.Errorf("seed is too long (max 64 bytes)")
	}

	device := cmd.StringArg("device")
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}
	device = adjustDevicePath(device)

	fileInfo, err := os.Stat(device)
	if err != nil {
		return nil, err
	}
	if !isBlockDevice(fileInfo) {
		return nil, fmt.Errorf("device %s is not a block device", device)
	}

	return &deviceConfig{
		seed:          seed,
		device:        device,
		info:          fileInfo,
		invertPattern: cmd.Bool("invert-pattern"),
	}, nil
}

func (a *baseApp) hashSeed(invert bool) {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte(a.cfg.seed))
	if invert {
		a.stream = &invertedReader{r: hasher.XOF()}
	} else {
		a.stream = hasher.XOF()
	}
}

type invertedReader struct {
	r io.Reader
}

func (ir *invertedReader) Read(p []byte) (n int, err error) {
	n, err = ir.r.Read(p)
	for i := 0; i < n; i++ {
		p[i] = ^p[i]
	}
	return n, err
}

func (a *baseApp) openDevice(flag int) error {
	device := a.cfg.device
	f, err := directio.OpenFile(device, flag, 0666)
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

func (a *baseApp) displayInfo(processName string) error {
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
	fmt.Printf("\nStarting %s process. This might take a while...\n", strings.ToLower(processName))
	return nil
}

type progressTracker struct {
	history    []progressRecord
	capacity   int
	firstPrint bool
}

func newProgressTracker(startTime time.Time, capacity int) *progressTracker {
	return &progressTracker{
		history:    []progressRecord{{timestamp: startTime, bytes: 0}},
		capacity:   capacity,
		firstPrint: true,
	}
}

func (pt *progressTracker) print(now time.Time, bytes int64) {
	pt.history = append(pt.history, progressRecord{timestamp: now, bytes: bytes})

	// keep last 5 seconds of history to display the current speed
	cutoff := now.Add(-5 * time.Second)
	keepFrom := 0
	maxDiscard := len(pt.history) - 1
	for keepFrom < maxDiscard && pt.history[keepFrom+1].timestamp.Before(cutoff) {
		keepFrom++
	}
	pt.history = pt.history[keepFrom:]

	// calculate displayed stats
	oldest := pt.history[0]
	elapsed := now.Sub(oldest.timestamp).Seconds()
	speed := 0.0
	if elapsed > 0 {
		speed = float64(bytes-oldest.bytes) / elapsed
	}
	percent := 0.0
	if pt.capacity > 0 {
		percent = float64(bytes) / float64(pt.capacity) * 100
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
		humanize.Bytes(uint64(bytes)),
		humanize.Bytes(uint64(pt.capacity)),
		fmt.Sprintf("%.2f%%", percent),
		humanize.Bytes(uint64(speed)) + "/s",
	})

	output := t.Render()
	numLines := strings.Count(output, "\n") + 1
	if !pt.firstPrint {
		fmt.Printf("\r\033[%dA", numLines)
	}
	fmt.Println(output)
	pt.firstPrint = false
}
