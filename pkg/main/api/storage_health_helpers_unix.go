//go:build !windows

package api

import "github.com/Kellerman81/go_media_downloader/pkg/main/scanner"

// getDiskUsageUnix gets disk usage on Unix-like systems (Linux, macOS, BSD).
// scanner.DiskUsage scales by f_frsize on Linux, where f_bsize is only the
// preferred I/O size and can differ from the unit the block counts use.
func getDiskUsageUnix(path string) (free uint64, total uint64, err error) {
	free, total, err = scanner.DiskUsage(path)
	if err != nil {
		return getDiskUsageFallback(path)
	}

	return free, total, nil
}
