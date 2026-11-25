// Command player is the desktop player of Go Media Downloader: it opens
// the portal's end-user interface (Library, Calendar, player) in an app
// window of Edge, Chrome, Chromium or Brave, with its own login. A start
// page in that window asks for the portal address on first start (or
// takes it from -url) and goes on to the portal.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if err := run(); err != nil {
		showError(err.Error())
		os.Exit(1)
	}
}

func run() error {
	urlFlag := flag.String("url", "", "portal address to use and remember")
	setupFlag := flag.Bool("setup", false, "ask for the portal address again")
	browserFlag := flag.String("browser", "", "browser to use and remember (path of Edge, Chrome or Chromium)")
	systemFlag := flag.Bool("system-browser", false, "open the portal in the default browser instead of an app window")
	flag.Parse()

	dir, err := appDir()
	if err != nil {
		return err
	}

	cfg := loadConfig(dir)

	if *browserFlag != "" {
		cfg.Browser = *browserFlag
	}

	if *urlFlag != "" {
		portal, err := normalizeURL(*urlFlag)
		if err != nil {
			return err
		}

		cfg.URL = portal
	}

	if *urlFlag != "" || *browserFlag != "" {
		if err := saveConfig(dir, cfg); err != nil {
			return err
		}
	}

	br := findBrowser()
	if cfg.Browser != "" {
		br = browserAt(cfg.Browser)
	}

	// The start page connects to the portal, or asks for its address first
	// (none yet, -setup, or the portal can't be reached). A Flatpak or Snap
	// browser can only read it inside its sandbox.
	pageDir := dir
	if br.Sandbox != "" && !*systemFlag {
		pageDir = br.Sandbox
	}

	page, err := startPage(pageDir, cfg.URL, *urlFlag != "", *setupFlag)
	if err != nil {
		return err
	}

	return open(br, dir, page, *systemFlag)
}

// open shows url in an app window (else the default browser) and waits
// until the window is closed.
func open(br browser, dir, url string, system bool) error {
	var cmd *exec.Cmd

	if br.Exe == "" || system {
		cmd = defaultBrowser(url)
	} else {
		cmd = appWindow(br, dir, url)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("can't start %s: %w", cmd.Path, err)
	}

	_ = cmd.Wait()

	return nil
}
