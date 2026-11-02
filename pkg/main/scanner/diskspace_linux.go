//go:build linux

package scanner

import "golang.org/x/sys/unix"

// statfsBlockSize returns the unit f_bavail is counted in. POSIX defines that
// as the fragment size (f_frsize); Bsize is only the preferred I/O size.
func statfsBlockSize(st *unix.Statfs_t) int64 {
	//nolint:unconvert // Frsize/Bsize types differ per architecture
	if fr := int64(st.Frsize); fr > 0 {
		return fr
	}

	//nolint:unconvert // Bsize type differs per architecture
	return int64(st.Bsize)
}
