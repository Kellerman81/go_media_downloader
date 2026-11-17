package main

import (
	"syscall"
	"unsafe"
)

// showError shows a message box: the Windows build has no console.
func showError(msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	text, _ := syscall.UTF16PtrFromString(msg)
	title, _ := syscall.UTF16PtrFromString("Go Media Player")

	const mbIconError = 0x10

	_, _, _ = user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
}
