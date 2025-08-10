//go:build windows
// +build windows

package api

import (
	"fmt"
	"syscall"
	"unsafe"
)

// getDiskUsageUnix provides a fallback implementation for Windows
func getDiskUsageUnix(path string) (free uint64, total uint64, err error) {
	// On Windows, we use the Windows API instead, so fallback to the generic implementation
	// This ensures the cross-platform getDiskUsage function works correctly by
	// routing Windows calls to the Windows-specific implementation
	return getDiskUsageFallback(path)
}

// getDiskUsageWindows gets disk usage on Windows systems
func getDiskUsageWindows(path string) (free uint64, total uint64, err error) {
	// Convert path to UTF16
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert path to UTF16: %v", err)
	}

	// Load kernel32.dll
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytes uint64
	var totalBytes uint64
	var totalFreeBytes uint64

	// Call GetDiskFreeSpaceExW
	ret, _, errno := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)))

	if ret == 0 {
		return 0, 0, fmt.Errorf("GetDiskFreeSpaceEx failed: %v", errno)
	}

	return freeBytes, totalBytes, nil
}