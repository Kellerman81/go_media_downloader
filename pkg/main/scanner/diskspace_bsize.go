//go:build !windows && !linux

package scanner

import "golang.org/x/sys/unix"

// statfsBlockSize returns the unit f_bavail is counted in. On the BSDs and
// macOS Statfs_t has no Frsize and Bsize is the fundamental block size.
func statfsBlockSize(st *unix.Statfs_t) int64 {
	//nolint:gosec,unconvert // Bsize type differs per platform
	return int64(st.Bsize)
}
