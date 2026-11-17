//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showError(msg string) {
	fmt.Fprintln(os.Stderr, "go-media-player:", msg)
}
