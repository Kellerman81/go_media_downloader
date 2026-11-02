//go:build !windows

package scanner

import (
	"errors"

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

	//nolint:gosec,unconvert // conversions vary by platform (Bavail types differ)
	return int64(st.Bavail) * statfsBlockSize(&st)
}

// isCrossDevice reports whether a rename failed because source and
// destination are on different filesystems.
func isCrossDevice(err error) bool {
	return errors.Is(err, unix.EXDEV)
}

// DiskUsage returns the bytes available to the current user and the total
// size of the filesystem containing path, in the same block unit as
// freeDiskSpace.
func DiskUsage(path string) (free, total uint64, err error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, err
	}

	bs := uint64(statfsBlockSize(&st)) //nolint:gosec // block size is positive

	//nolint:gosec,unconvert // Bavail/Blocks types vary by platform
	return uint64(st.Bavail) * bs, uint64(st.Blocks) * bs, nil
}
