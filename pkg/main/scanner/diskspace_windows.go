//go:build windows

package scanner

import (
	"errors"
	"math"

	"golang.org/x/sys/windows"
)

// freeDiskSpace returns the bytes available to the current user on the volume
// containing path, or -1 when it cannot be determined (callers fail open).
func freeDiskSpace(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1
	}

	var free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, nil, nil); err != nil {
		return -1
	}

	if free > math.MaxInt64 {
		return math.MaxInt64
	}

	return int64(free)
}

// isCrossDevice reports whether a rename failed because source and
// destination are on different volumes.
func isCrossDevice(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
