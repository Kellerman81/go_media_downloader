package api

import (
	"net"
	"testing"

	"golang.org/x/net/ipv4"
)

// TestSSDPAnswerIP: a search from this host over loopback gets a loopback
// address (the announcing interfaces don't include loopback); one from an
// unknown network on no announcing interface gets none.
func TestSSDPAnswerIP(t *testing.T) {
	s := &ssdpServer{}

	if ip := s.answerIP(&ipv4.ControlMessage{IfIndex: 1}, net.IPv4(127, 0, 0, 1)); !ip.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("loopback search answered with %v", ip)
	}

	if ip := s.answerIP(&ipv4.ControlMessage{IfIndex: 99}, net.IPv4(192, 168, 7, 7)); ip != nil {
		t.Errorf("search on no announcing interface answered with %v", ip)
	}
}
