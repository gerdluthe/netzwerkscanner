//go:build !windows

package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// openURLHook is set by the GUI (Fyne) so links can be opened on every
// platform, including Android.
var openURLHook func(string) error

// icmpPing sends one ICMP echo request. It first tries an unprivileged
// ICMP datagram socket (macOS, Android, most Linux distributions) and
// falls back to a raw socket when running with root rights.
func icmpPing(ip net.IP, timeoutMs uint32) (PingResult, error) {
	privileged := false
	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		c, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			return PingResult{}, errNoIcmp
		}
		privileged = true
	}
	defer c.Close()

	p4 := c.IPv4PacketConn()
	if p4 != nil {
		p4.SetControlMessage(ipv4.FlagTTL, true)
	}
	seq := rand.Intn(0xffff)
	msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{ID: os.Getpid() & 0xffff, Seq: seq, Data: []byte("NetzScanPingData32bytes.........")}}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return PingResult{}, err
	}
	var dst net.Addr = &net.UDPAddr{IP: ip}
	if privileged {
		dst = &net.IPAddr{IP: ip}
	}
	start := time.Now()
	if _, err := c.WriteTo(wb, dst); err != nil {
		return PingResult{}, nil
	}
	c.SetReadDeadline(start.Add(time.Duration(timeoutMs) * time.Millisecond))
	rb := make([]byte, 1500)
	for {
		var n, ttl int
		var peer net.Addr
		if p4 != nil {
			var cm *ipv4.ControlMessage
			n, cm, peer, err = p4.ReadFrom(rb)
			if cm != nil {
				ttl = cm.TTL
			}
		} else {
			n, peer, err = c.ReadFrom(rb)
		}
		if err != nil {
			return PingResult{}, nil
		}
		if !sameIP(peer, ip) {
			continue
		}
		data := rb[:n]
		// some systems deliver the IP header as well
		if len(data) > 20 && data[0]>>4 == 4 {
			hl := int(data[0]&0x0f) * 4
			if ttl == 0 {
				ttl = int(data[8])
			}
			data = data[hl:]
		}
		rm, err := icmp.ParseMessage(1, data)
		if err != nil || rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		if echo, ok := rm.Body.(*icmp.Echo); ok && echo.Seq == seq {
			return PingResult{OK: true, RTT: uint32(time.Since(start).Milliseconds()), TTL: uint8(ttl)}, nil
		}
	}
}

func sameIP(a net.Addr, ip net.IP) bool {
	switch v := a.(type) {
	case *net.UDPAddr:
		return v.IP.Equal(ip)
	case *net.IPAddr:
		return v.IP.Equal(ip)
	}
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		host = a.String()
	}
	return net.ParseIP(host).Equal(ip)
}

// enumShares is only implemented on Windows (NetShareEnum).
func enumShares(host string) ([]Share, error) {
	return nil, fmt.Errorf("share enumeration is only available on Windows")
}

func sharesSupported() bool { return false }

func shellOpen(target string, args string) {
	if openURLHook != nil && strings.Contains(target, "://") {
		if openURLHook(target) == nil {
			return
		}
	}
	switch runtime.GOOS {
	case "darwin":
		if strings.HasPrefix(target, `\\`) {
			target = "smb:" + strings.ReplaceAll(target, `\`, "/")
		}
		exec.Command("open", target).Start()
	case "android":
		// handled by openURLHook
	default:
		exec.Command("xdg-open", target).Start()
	}
}
