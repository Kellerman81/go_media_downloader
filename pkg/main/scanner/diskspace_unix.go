//go:build !windows

package scanner

import (
	"golang.org/x/sys/unix"
)

// freeDiskSpace returns the bytes available to the current user on the
// filesystem containing path, or -1 when it cannot be determined (callers
// fail open).
func freeDiskSpace(path string) int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return -1
	}

	//nolint:gosec,unconvert // conversions vary by platform (Bavail/Bsize types differ)
	return int64(st.Bavail) * int64(st.Bsize)
}
