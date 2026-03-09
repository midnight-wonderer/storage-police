//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Constants from Win32 API
const (
	IOCTL_DISK_GET_DRIVE_GEOMETRY_EX = 0x000700A0
	IOCTL_DISK_GET_LENGTH_INFO       = 0x0007405C
)

type diskGeometryEx struct {
	Geometry diskGeometry
	DiskSize int64
	Data     [1]byte
}

type diskGeometry struct {
	Cylinders         int64
	MediaType         uint32
	TracksPerCylinder uint32
	SectorsPerTrack   uint32
	BytesPerSector    uint32
}

func getBlockDeviceSize(f *os.File) (int, error) {
	var geometry diskGeometryEx
	var returned uint32
	err := windows.DeviceIoControl(
		windows.Handle(f.Fd()),
		IOCTL_DISK_GET_DRIVE_GEOMETRY_EX,
		nil,
		0,
		(*byte)(unsafe.Pointer(&geometry)),
		uint32(unsafe.Sizeof(geometry)),
		&returned,
		nil,
	)
	if err != nil {
		// Fallback for some volumes/partitions
		var length int64
		err = windows.DeviceIoControl(
			windows.Handle(f.Fd()),
			IOCTL_DISK_GET_LENGTH_INFO,
			nil,
			0,
			(*byte)(unsafe.Pointer(&length)),
			uint32(unsafe.Sizeof(length)),
			&returned,
			nil,
		)
		if err != nil {
			return 0, fmt.Errorf("could not get size of block device: %w", err)
		}
		return int(length), nil
	}
	return int(geometry.DiskSize), nil
}

func isBlockDevice(info os.FileInfo) bool {
	// On Windows, os.Stat on device paths often doesn't give ModeDevice.
	// We rely on the path prefix check in parseDeviceConfig and the fact that
	// openDevice will fail if it's not accessible.
	// For simplicity, we return true if it's not a directory.
	return !info.IsDir()
}

// Windows-specific path adjustment
func adjustDevicePath(path string) string {
	// If it's just a number, assume PhysicalDriveN
	if _, err := fmt.Sscanf(path, "%d", new(int)); err == nil {
		return `\\.\PhysicalDrive` + path
	}
	// If it's something like D:, make it \\.\D:
	if len(path) == 2 && path[1] == ':' {
		return `\\.\` + path
	}
	// If it doesn't start with \\.\ but looks like it might be a disk name
	if !strings.HasPrefix(path, `\\.\`) && (strings.HasPrefix(path, "PhysicalDrive") || (len(path) > 1 && path[1] == ':')) {
		return `\\.\` + path
	}
	return path
}

func isNoSpaceError(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_EOF) || errors.Is(err, windows.Errno(38))
}
