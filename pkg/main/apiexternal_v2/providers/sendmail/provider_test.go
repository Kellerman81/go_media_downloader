package sendmail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestSplitRecipients(t *testing.T) {
	got := splitRecipients([]string{" a@x.org , ,b@x.org", "c@x.org\r\n", ""})
	want := []string{"a@x.org", "b@x.org", "c@x.org"}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitRecipients = %q, want %q", got, want)
	}
}

func TestBuildMessageHeaders(t *testing.T) {
	msg := string(buildMessage("me@example.org", []string{"a@x.org"}, "Grüße", "line1\nline2", time.Unix(0, 0).UTC()))

	for _, want := range []string{
		"Subject: =?utf-8?q?Gr=C3=BC=C3=9Fe?=\r\n",
		"Date: Thu, 01 Jan 1970 00:00:00 +0000\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n",
		"@example.org>\r\n",
		"line1\r\nline2",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

// fakeSMTP is a minimal SMTP server without STARTTLS/AUTH that records the
// commands it receives.
func fakeSMTP(t *testing.T) (port int, cmds chan []string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { ln.Close() })

	cmds = make(chan []string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var seen []string

		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }

		write("220 fake ESMTP")

		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				cmds <- seen
				return
			}

			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false

					write("250 queued")
				}

				continue
			}

			seen = append(seen, strings.ToUpper(strings.SplitN(line, " ", 2)[0]))

			switch {
			case strings.HasPrefix(line, "EHLO"):
				write("250-fake")
				write("250 8BITMIME")
			case strings.HasPrefix(line, "DATA"):
				inData = true

				write("354 go ahead")
			case strings.HasPrefix(line, "QUIT"):
				write("221 bye")

				cmds <- seen

				return
			default:
				write("250 ok")
			}
		}
	}()

	return ln.Addr().(*net.TCPAddr).Port, cmds
}

func TestSendWithoutCredentialsSkipsAuth(t *testing.T) {
	port, cmds := fakeSMTP(t)

	p := NewProvider("127.0.0.1", port, "me@example.org", []string{"a@x.org, b@x.org"}, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := p.SendNotification(ctx, apiexternal_v2.NotificationRequest{Title: "T", Message: "M"})
	if err != nil || !resp.Success {
		t.Fatalf("SendNotification: %v", err)
	}

	got := strings.Join(<-cmds, " ")
	if got != "EHLO MAIL RCPT RCPT DATA QUIT" {
		t.Errorf("commands = %q", got)
	}
}
