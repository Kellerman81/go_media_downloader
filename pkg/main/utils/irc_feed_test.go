package utils

import (
	"bufio"
	"context"
	"errors"
	"maps"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// fakeIRCServer accepts one client and hands the test a reader of the
// client's lines plus the raw connection to write server lines to.
func fakeIRCServer(t *testing.T) (addr string, accept func() (net.Conn, *bufio.Reader)) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { ln.Close() })

	return ln.Addr().String(), func() (net.Conn, *bufio.Reader) {
		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() { c.Close() })

		return c, bufio.NewReader(c)
	}
}

// expectLine reads client lines until one has the prefix, failing on timeout.
func expectLine(t *testing.T, c net.Conn, r *bufio.Reader, prefix string) string {
	t.Helper()

	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("waiting for %q: %v", prefix, err)
		}

		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
}

func newTestIRCSession() *ircSession {
	return &ircSession{key: "k", listName: "l", lastDrain: time.Now(), cancel: func() {}}
}

func TestIRCConnectNickCollisionJoinAndBuffer(t *testing.T) {
	old := ircReadIdle
	ircReadIdle = 300 * time.Millisecond

	t.Cleanup(func() { ircReadIdle = old })

	addr, accept := fakeIRCServer(t)
	cl := &config.ListsConfig{IRCServer: addr, IRCNick: "bot", IRCChannels: []string{"#Announce"}}
	s := newTestIRCSession()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- s.connectAndListen(ctx, cl) }()

	c, r := accept()
	expectLine(t, c, r, "NICK bot")
	expectLine(t, c, r, "USER ")

	c.Write([]byte(":srv 433 * bot :Nickname is already in use\r\n"))
	expectLine(t, c, r, "NICK bot_")

	c.Write([]byte(":srv 001 bot_ :Welcome\r\n"))
	expectLine(t, c, r, "JOIN #Announce")

	// Coloured announce, then one split across a read deadline.
	c.Write([]byte(":ann!u@h PRIVMSG #announce :\x02New:\x02 \x0304,01Movie.2020\x03 1080p\r\n"))
	c.Write([]byte(":ann!u@h PRIVMSG #announce :Split.Li"))
	time.Sleep(450 * time.Millisecond) // past ircReadIdle: client PINGs
	expectLine(t, c, r, "PING")
	c.Write([]byte("ne.2021\r\n"))

	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		n := len(s.buffer)
		s.mu.Unlock()

		if n >= 2 || time.Now().After(deadline) {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	if got := s.drain(); !slices.Equal(got, []string{"New: Movie.2020 1080p", "Split.Line.2021"}) {
		t.Fatalf("buffer = %q", got)
	}

	cancel()
	<-done
}

// Cancel must end the session without waiting for the read deadline.
func TestIRCConnectCancelClosesPromptly(t *testing.T) {
	old := ircReadIdle
	ircReadIdle = time.Hour

	t.Cleanup(func() { ircReadIdle = old })

	addr, accept := fakeIRCServer(t)
	cl := &config.ListsConfig{IRCServer: addr, IRCNick: "bot", IRCChannels: []string{"#a"}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- newTestIRCSession().connectAndListen(ctx, cl) }()

	c, r := accept()
	expectLine(t, c, r, "USER ")
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connectAndListen did not return after cancel")
	}
}

func TestIRCConnectUnansweredPing(t *testing.T) {
	old := ircReadIdle
	ircReadIdle = 150 * time.Millisecond

	t.Cleanup(func() { ircReadIdle = old })

	addr, accept := fakeIRCServer(t)
	cl := &config.ListsConfig{IRCServer: addr, IRCNick: "bot", IRCChannels: []string{"#a"}}
	done := make(chan error, 1)

	go func() { done <- newTestIRCSession().connectAndListen(context.Background(), cl) }()

	c, _ := accept()
	c.Write([]byte(":srv 001 bot :Welcome\r\n"))

	select {
	case err := <-done:
		if !errors.Is(err, errIRCNoResponse) {
			t.Fatalf("err = %v; want errIRCNoResponse", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("half-open connection not detected")
	}
}

// With a NickServ password the channels are joined only after NickServ
// confirms the login (+r channels reject unidentified users).
func TestIRCConnectJoinWaitsForNickServ(t *testing.T) {
	addr, accept := fakeIRCServer(t)
	cl := &config.ListsConfig{
		IRCServer: addr, IRCNick: "bot", IRCChannels: []string{"#a"},
		IRCNickServPassword: "secret", IRCInviteCommand: "PRIVMSG Inviter :invite",
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- newTestIRCSession().connectAndListen(ctx, cl) }()

	c, r := accept()
	expectLine(t, c, r, "USER ")
	c.Write([]byte(":srv 001 bot :Welcome\r\n"))
	expectLine(t, c, r, "PRIVMSG NickServ :IDENTIFY secret")

	// Unrelated traffic must not trigger the join.
	c.Write([]byte(":NickServ!s@services NOTICE bot :This nickname is registered.\r\n"))
	c.Write([]byte(":srv 375 bot :- MOTD\r\n"))

	_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if line, err := r.ReadString('\n'); err == nil {
		t.Fatalf("client sent %q before NickServ confirmed", line)
	}

	c.Write([]byte(":NickServ!s@services NOTICE bot :You are now identified for \x02bot\x02.\r\n"))
	expectLine(t, c, r, "PRIVMSG Inviter :invite")
	expectLine(t, c, r, "JOIN #a")

	cancel()
	<-done
}

// Without a confirmation the join still happens after ircIdentifyTimeout.
func TestIRCConnectJoinAfterNickServTimeout(t *testing.T) {
	old := ircIdentifyTimeout
	ircIdentifyTimeout = 200 * time.Millisecond

	t.Cleanup(func() { ircIdentifyTimeout = old })

	addr, accept := fakeIRCServer(t)
	cl := &config.ListsConfig{IRCServer: addr, IRCNick: "bot", IRCChannels: []string{"#a"}, IRCNickServPassword: "pw"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- newTestIRCSession().connectAndListen(ctx, cl) }()

	c, r := accept()
	c.Write([]byte(":srv 001 bot :Welcome\r\n"))
	expectLine(t, c, r, "PRIVMSG NickServ :IDENTIFY pw")

	start := time.Now()
	expectLine(t, c, r, "JOIN #a")

	if waited := time.Since(start); waited < 150*time.Millisecond {
		t.Fatalf("joined after %v, before the identify timeout", waited)
	}

	cancel()
	<-done
}

func TestIRCIdentifyResult(t *testing.T) {
	tests := []struct {
		line     string
		done, ok bool
	}{
		{":srv 900 bot bot!u@h bot :You are now logged in as bot", true, true},
		{":srv 904 bot :SASL authentication failed", true, false},
		{":NickServ!NickServ@services. NOTICE bot :You are now identified for bot.", true, true},
		{":nickserv!s@services NOTICE bot :Password accepted - you are now recognized.", true, true},
		{":NickServ!s@services NOTICE bot :Invalid password for bot.", true, false},
		{":NickServ!s@services NOTICE bot :This nickname is registered.", false, false},
		{":evil!u@h NOTICE bot :You are now identified", false, false},
		{":srv 001 bot :Welcome", false, false},
	}

	for _, tt := range tests {
		if done, ok := ircIdentifyResult(tt.line); done != tt.done || ok != tt.ok {
			t.Errorf("ircIdentifyResult(%q) = %v, %v; want %v, %v", tt.line, done, ok, tt.done, tt.ok)
		}
	}
}

func TestStripIRCFormatting(t *testing.T) {
	tests := map[string]string{
		"plain":                  "plain",
		"\x02bold\x02":           "bold",
		"\x034red\x03 x":         "red x",
		"\x0312,05both\x0f done": "both done",
		"\x03,5comma":            ",5comma",
		"100\x0399% \x1ditalic":  "100% italic",
	}

	for in, want := range tests {
		if got := stripIRCFormatting(in); got != want {
			t.Errorf("stripIRCFormatting(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestIRCRequeue(t *testing.T) {
	s := &ircSession{key: "test\x00list", listName: "list"}

	s.appendLine("a")
	s.appendLine("b")

	lines := s.drain()
	s.appendLine("c") // arrived while the run was importing

	s.requeue(lines, false)

	if got := s.drain(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("requeued order = %v, want [a b c]", got)
	}

	if len(s.attempts) != 0 {
		t.Fatalf("uncharged requeue recorded attempts: %v", s.attempts)
	}

	// A line that fails every run is dropped after ircMaxAttempts.
	for i := 1; i <= ircMaxAttempts; i++ {
		s.requeue([]string{"poison"}, true)

		got := s.drain()
		if i < ircMaxAttempts && !slices.Equal(got, []string{"poison"}) {
			t.Fatalf("attempt %d: buffer = %v, want [poison]", i, got)
		}

		if i == ircMaxAttempts && len(got) != 0 {
			t.Fatalf("poison line still requeued after %d attempts: %v", i, got)
		}
	}

	if len(s.attempts) != 0 {
		t.Fatalf("dropped line left an attempt count: %v", s.attempts)
	}

	// A successful run clears the failure count.
	s.requeue([]string{"x"}, true)
	s.commit(s.drain())

	if len(s.attempts) != 0 {
		t.Fatalf("commit left attempts: %v", s.attempts)
	}
}

func TestIRCRequeueBounded(t *testing.T) {
	s := &ircSession{}

	old := make([]string, 10)
	for i := range old {
		old[i] = "old" + strconv.Itoa(i)
	}

	for i := range ircBufferMax {
		s.appendLine("new" + strconv.Itoa(i))
	}

	s.requeue(old, true)

	if n := len(s.buffer); n != ircBufferMax {
		t.Fatalf("buffer holds %d lines, want %d", n, ircBufferMax)
	}

	if s.buffer[0] != "new0" {
		t.Fatalf("oldest lines not dropped first: buffer starts with %q", s.buffer[0])
	}

	if len(s.attempts) != 0 {
		t.Fatalf("attempts kept for lines dropped by the cap: %d", len(s.attempts))
	}
}

func TestNormaliseImdbID(t *testing.T) {
	for in, want := range map[string]string{"": "", "   ": "", "0111161": "tt0111161", " tt0111161 ": "tt0111161"} {
		if got := normaliseImdbID(in); got != want {
			t.Errorf("normaliseImdbID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNamedSubmatches(t *testing.T) {
	re := regexp.MustCompile(`^NEW: (?P<title>.+?) \[(?P<imdb>tt\d+)\](?: (\d+)p)?$`)

	got := namedSubmatches(re, "NEW: Some Movie [tt0111161] 1080p")
	want := map[string]string{"title": "Some Movie", "imdb": "tt0111161"}

	if !maps.Equal(got, want) {
		t.Errorf("namedSubmatches = %v, want %v", got, want)
	}

	if got := namedSubmatches(re, "unrelated line"); got != nil {
		t.Errorf("no match = %v, want nil", got)
	}
}
