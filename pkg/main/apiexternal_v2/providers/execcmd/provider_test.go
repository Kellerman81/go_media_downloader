package execcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func script(t *testing.T, body string) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "notify.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}

	return p
}

func TestSendNotification(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	p := NewProvider(script(t, `printf '%s|%s|%s|%s' "$1" "$2" "$GMD_TITLE" "$GMD_MESSAGE" > `+out+"\n"))

	// Shell syntax in the text must arrive literally.
	title, msg := "Heat; rm -rf /tmp/x", "$(id) `id`"
	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: title, Message: msg}); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(out)
	if want := title + "|" + msg + "|" + title + "|" + msg; string(b) != want {
		t.Errorf("program got %q, want %q", b, want)
	}

	failing := NewProvider(script(t, "echo broken >&2\nexit 3\n"))

	_, err := failing.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Message: "m"})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("failure not reported with output: %v", err)
	}
}

func TestTestConnection(t *testing.T) {
	if err := NewProvider(script(t, "exit 0\n")).TestConnection(context.Background()); err != nil {
		t.Errorf("executable rejected: %v", err)
	}

	plain := filepath.Join(t.TempDir(), "plain")
	os.WriteFile(plain, []byte("x"), 0o644)

	for _, bad := range []string{plain, t.TempDir(), "/does/not/exist"} {
		if NewProvider(bad).TestConnection(context.Background()) == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
