//go:build !windows
// +build !windows

package api

// getDiskUsageWindows provides a stub implementation for non-Windows systems  
func getDiskUsageWindows(path string) (free uint64, total uint64, err error) {
	// On non-Windows systems, fallback to generic implementation
	return getDiskUsageFallback(path)
}