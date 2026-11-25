//go:build !windows

package api

import "syscall"

// reuseAddr lets the SSDP socket share UDP port 1900 with other UPnP
// programs on the host (Plex, minidlna).
func reuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
