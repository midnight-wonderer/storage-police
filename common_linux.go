//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func getBlockDeviceSize(f *os.File) (int, error) {
	size, err := unix.IoctlGetInt(int(f.Fd()), unix.BLKGETSIZE64)
	if err != nil {
		return 0, fmt.Errorf("could not get size of block device: %w", err)
	}
	return size, nil
}

func isBlockDevice(info os.FileInfo) bool {
	mode := info.Mode()
	return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0
}

func adjustDevicePath(path string) string {
	return path
}

func isNoSpaceError(err error) bool {
	return errors.Is(err, unix.ENOSPC)
}
