//go:build darwin

package main

import (
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var arpRe = regexp.MustCompile(` at ([0-9a-fA-F]{1,2}(?::[0-9a-fA-F]{1,2}){5}) `)

// arpLookup reads the system ARP cache (filled by the preceding ping).
func arpLookup(ip net.IP) (net.HardwareAddr, bool) {
	for attempt := 0; attempt < 2; attempt++ {
		out, err := exec.Command("/usr/sbin/arp", "-n", ip.String()).Output()
		if err == nil {
			if m := arpRe.FindStringSubmatch(string(out)); m != nil {
				parts := strings.Split(m[1], ":")
				for i, p := range parts {
					if len(p) == 1 {
						parts[i] = "0" + p
					}
				}
				if hw, err := net.ParseMAC(strings.Join(parts, ":")); err == nil {
					return hw, true
				}
			}
		}
		// trigger an ARP request and retry once
		if c, err := net.DialTimeout("udp4", net.JoinHostPort(ip.String(), "9"), 200*time.Millisecond); err == nil {
			c.Write([]byte{0})
			c.Close()
			time.Sleep(150 * time.Millisecond)
		}
	}
	return nil, false
}
