//go:build !windows
// +build !windows

package api

import (
	"syscall"
)

// getDiskUsageUnix gets disk usage on Unix-like systems (Linux, macOS, BSD)
func getDiskUsageUnix(path string) (free uint64, total uint64, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(path, &stat)
	if err != nil {
		// If the syscall fails, try the fallback method
		return getDiskUsageFallback(path)
	}

	// Calculate free and total bytes
	blockSize := uint64(stat.Bsize)
	freeBlocks := uint64(stat.Bavail) // Available to non-privileged user
	totalBlocks := uint64(stat.Blocks)

	freeBytes := freeBlocks * blockSize
	totalBytes := totalBlocks * blockSize

	return freeBytes, totalBytes, nil
}