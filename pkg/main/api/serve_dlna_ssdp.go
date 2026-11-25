package api

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"golang.org/x/net/ipv4"
)

// SSDP announces the DLNA server in the local network (multicast
// 239.255.255.250:1900): NOTIFY messages every few minutes and answers to
// the devices' M-SEARCH. It runs while serve_dlna is on;
// StartDLNAAnnouncements starts and stops it when the setting changes.

const (
	ssdpAddr   = "239.255.255.250:1900"
	ssdpMaxAge = 1800
)

var ssdpTypes = []string{
	"upnp:rootdevice",
	"urn:schemas-upnp-org:device:MediaServer:1",
	"urn:schemas-upnp-org:service:ContentDirectory:1",
	"urn:schemas-upnp-org:service:ConnectionManager:1",
}

type ssdpServer struct {
	conn   *ipv4.PacketConn
	ifaces []net.Interface
	done   chan struct{}
	wg     sync.WaitGroup
}

var ssdpOnce sync.Once

// StartDLNAAnnouncements announces the DLNA server while serve_dlna is
// on (checked every 30 seconds).
func StartDLNAAnnouncements() {
	ssdpOnce.Do(func() {
		go func() {
			var srv *ssdpServer

			for {
				on := config.GetSettingsGeneral().ServeDLNA

				switch {
				case on && srv == nil:
					s, err := startSSDP()
					if err != nil {
						logger.Logtype("error", 0).Err(err).Msg("DLNA announcements (SSDP, UDP port 1900) not started")
					} else {
						srv = s
						logger.Logtype("info", 0).Int("interfaces", len(s.ifaces)).Msg("DLNA server announced")
					}
				case !on && srv != nil:
					srv.stop()
					srv = nil
				}

				time.Sleep(30 * time.Second)
			}
		}()
	})
}

func startSSDP() (*ssdpServer, error) {
	lc := net.ListenConfig{Control: func(_, _ string, rc syscall.RawConn) error {
		var serr error
		if err := rc.Control(func(fd uintptr) { serr = reuseAddr(fd) }); err != nil {
			return err
		}

		return serr
	}}

	pc, err := lc.ListenPacket(context.Background(), "udp4", "0.0.0.0:1900")
	if err != nil {
		return nil, err
	}

	s := &ssdpServer{conn: ipv4.NewPacketConn(pc), done: make(chan struct{})}
	group := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250)}

	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagMulticast == 0 || ifc.Flags&net.FlagLoopback != 0 || ifaceIPv4(&ifc) == nil {
			continue
		}

		if err := s.conn.JoinGroup(&ifc, group); err == nil {
			s.ifaces = append(s.ifaces, ifc)
		}
	}

	if len(s.ifaces) == 0 {
		pc.Close()
		return nil, net.ErrClosed
	}

	_ = s.conn.SetControlMessage(ipv4.FlagInterface, true)
	_ = s.conn.SetMulticastTTL(2)

	s.wg.Add(2)

	go s.listen()
	go s.announce()

	return s, nil
}

func (s *ssdpServer) stop() {
	close(s.done)
	s.notify("ssdp:byebye")
	s.conn.Close()
	s.wg.Wait()
}

// ifaceIPv4 is the first IPv4 address of an interface.
func ifaceIPv4(ifc *net.Interface) net.IP {
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			return n.IP.To4()
		}
	}

	return nil
}

// location is the device description URL announced on an interface.
func ssdpLocation(ip net.IP) string {
	if base := strings.TrimRight(config.GetSettingsGeneral().DLNAURL, "/"); base != "" {
		return base + "/dlna/device.xml"
	}

	port := config.GetSettingsGeneral().WebPort
	if port == "" {
		port = "9090"
	}

	return "http://" + net.JoinHostPort(ip.String(), port) + "/dlna/device.xml"
}

// usn is the unique service name of a notification type.
func ssdpUSN(nt string) string {
	if nt == "uuid:"+dlnaUUID() {
		return nt
	}

	return "uuid:" + dlnaUUID() + "::" + nt
}

func ssdpServerHeader() string { return "Linux/1.0 UPnP/1.0 MediaDownloader/1.0" }

// announce sends NOTIFY alive on start and then regularly.
func (s *ssdpServer) announce() {
	defer s.wg.Done()

	t := time.NewTicker(ssdpMaxAge / 3 * time.Second)
	defer t.Stop()

	for {
		s.notify("ssdp:alive")

		select {
		case <-s.done:
			return
		case <-t.C:
		}
	}
}

func (s *ssdpServer) notify(nts string) {
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

	for i := range s.ifaces {
		ifc := &s.ifaces[i]

		ip := ifaceIPv4(ifc)
		if ip == nil || s.conn.SetMulticastInterface(ifc) != nil {
			continue
		}

		for _, nt := range append([]string{"uuid:" + dlnaUUID()}, ssdpTypes...) {
			msg := "NOTIFY * HTTP/1.1\r\nHOST: " + ssdpAddr + "\r\nCACHE-CONTROL: max-age=" + strconv.Itoa(ssdpMaxAge) +
				"\r\nLOCATION: " + ssdpLocation(ip) + "\r\nNT: " + nt + "\r\nNTS: " + nts + "\r\nSERVER: " + ssdpServerHeader() +
				"\r\nUSN: " + ssdpUSN(nt) + "\r\n\r\n"
			_, _ = s.conn.WriteTo([]byte(msg), nil, dst)
		}
	}
}

// listen answers the devices' searches.
func (s *ssdpServer) listen() {
	defer s.wg.Done()

	buf := make([]byte, 2048)

	for {
		n, cm, src, err := s.conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.done:
				return
			default:
			}

			time.Sleep(time.Second)

			continue
		}

		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf[:n])))
		if err != nil || req.Method != "M-SEARCH" || !strings.Contains(req.Header.Get("Man"), "ssdp:discover") {
			continue
		}

		addr, ok := src.(*net.UDPAddr)
		if !ok || !addr.IP.IsPrivate() && !addr.IP.IsLinkLocalUnicast() && !addr.IP.IsLoopback() {
			continue
		}

		st := req.Header.Get("St")

		var types []string

		switch {
		case st == "ssdp:all":
			types = append([]string{"uuid:" + dlnaUUID()}, ssdpTypes...)
		case st == "uuid:"+dlnaUUID():
			types = []string{st}
		default:
			for _, t := range ssdpTypes {
				if t == st {
					types = []string{t}
				}
			}
		}

		ip := s.answerIP(cm, addr.IP)
		if ip == nil {
			continue
		}

		for _, t := range types {
			msg := "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=" + strconv.Itoa(ssdpMaxAge) + "\r\nDATE: " +
				time.Now().UTC().Format(http.TimeFormat) + "\r\nEXT:\r\nLOCATION: " + ssdpLocation(ip) + "\r\nSERVER: " +
				ssdpServerHeader() + "\r\nST: " + t + "\r\nUSN: " + ssdpUSN(t) + "\r\n\r\n"
			_, _ = s.conn.WriteTo([]byte(msg), nil, addr)
		}
	}
}

// answerIP is the server's address on the interface a search came in on
// (one in the asking device's network when it has several). A search from
// this host itself over loopback is answered with loopback: the announcing
// interfaces don't include it.
func (s *ssdpServer) answerIP(cm *ipv4.ControlMessage, peer net.IP) net.IP {
	if peer.IsLoopback() {
		return net.IPv4(127, 0, 0, 1).To4()
	}

	for i := range s.ifaces {
		ifc := &s.ifaces[i]
		if cm != nil && cm.IfIndex != 0 && ifc.Index != cm.IfIndex {
			continue
		}

		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.Contains(peer) {
				return n.IP.To4()
			}
		}

		if cm != nil && cm.IfIndex != 0 {
			return ifaceIPv4(ifc)
		}
	}

	return nil
}
