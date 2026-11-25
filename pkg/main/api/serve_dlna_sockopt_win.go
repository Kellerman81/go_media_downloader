//go:build windows

package api

import "syscall"

// reuseAddr lets the SSDP socket share UDP port 1900 with other UPnP
// programs (Windows' own SSDP service).
func reuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
