//go:build darwin

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	DKIOCGETBLOCKSIZE  = 0x40046418
	DKIOCGETBLOCKCOUNT = 0x40086419
)

func getBlockDeviceSize(f *os.File) (int, error) {
	// DKIOCGETBLOCKCOUNT returns the number of 512-byte blocks
	// DKIOCGETBLOCKSIZE returns the size of each block

	var count uint64
	_, _, err := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(DKIOCGETBLOCKCOUNT), uintptr(unsafe.Pointer(&count)))
	if err != 0 {
		return 0, fmt.Errorf("could not get block count: %w", err)
	}

	var size uint32
	_, _, err = unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(DKIOCGETBLOCKSIZE), uintptr(unsafe.Pointer(&size)))
	if err != 0 {
		return 0, fmt.Errorf("could not get block size: %w", err)
	}

	return int(count * uint64(size)), nil
}

func isBlockDevice(info os.FileInfo) bool {
	mode := info.Mode()
	// On macOS, most disks are character devices (e.g. /dev/rdisk1)
	return mode&os.ModeDevice != 0
}

func adjustDevicePath(path string) string {
	// For raw disk access on macOS, users should use /dev/rdiskX instead of /dev/diskX
	// We can automatically suggest it if we detect a raw disk path format
	if strings.HasPrefix(path, "/dev/disk") {
		return strings.Replace(path, "/dev/disk", "/dev/rdisk", 1)
	}
	return path
}

func isNoSpaceError(err error) bool {
	return errors.Is(err, unix.ENOSPC)
}
