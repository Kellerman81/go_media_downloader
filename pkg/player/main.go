// Command player is the desktop player of Go Media Downloader: it opens
// the portal's end-user interface (Library, Calendar, player) in an app
// window of Edge, Chrome or Chromium, with its own login. The portal
// address is asked for on first start and kept in config.json.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
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

	// What to open: the portal, or the setup page when there is no address
	// yet, it was asked for, or the portal can't be reached.
	problem := ""
	if cfg.URL != "" && !*setupFlag {
		if err := reachable(cfg.URL); err != nil {
			problem = err.Error()
		}
	}

	if cfg.URL != "" && !*setupFlag && problem == "" {
		return open(br, dir, appURL(cfg.URL), *systemFlag)
	}

	page, saved, stop, err := setup(dir, cfg, problem)
	if err != nil {
		return err
	}
	defer stop()

	if br.Exe == "" || *systemFlag {
		// The default browser opens the setup page and goes on to the portal
		// by itself; wait until it was saved.
		if err := defaultBrowser(page).Start(); err != nil {
			return err
		}

		select {
		case <-saved:
			time.Sleep(time.Second) // let the redirect reach the browser
		case <-time.After(30 * time.Minute):
		}

		return nil
	}

	cmd := appWindow(br, dir, page)
	if err := cmd.Start(); err != nil {
		return err
	}

	// Serve the setup page while the window is open.
	_ = cmd.Wait()

	return nil
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

var errUnreachable = errors.New("the portal can't be reached")

// reachable checks that the portal answers (any HTTP answer counts: the
// login page, a redirect).
func reachable(portal string) error {
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(portal + "/api/admin/login")
	if err != nil {
		return fmt.Errorf("%w at %s - check the address and that the server runs", errUnreachable, portal)
	}

	resp.Body.Close()

	return nil
}
