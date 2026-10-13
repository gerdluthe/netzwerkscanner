//go:build linux

package main

import (
	"bufio"
	"net"
	"os"
	"strings"
	"time"
)

// arpLookup reads /proc/net/arp (Linux). On Android 10+ this file is not
// readable for apps, so MAC addresses stay empty there.
func arpLookup(ip net.IP) (net.HardwareAddr, bool) {
	for attempt := 0; attempt < 2; attempt++ {
		if hw, ok := readProcArp(ip); ok {
			return hw, true
		}
		if c, err := net.DialTimeout("udp4", net.JoinHostPort(ip.String(), "9"), 200*time.Millisecond); err == nil {
			c.Write([]byte{0})
			c.Close()
			time.Sleep(150 * time.Millisecond)
		}
	}
	return nil, false
}

func readProcArp(ip net.IP) (net.HardwareAddr, bool) {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	want := ip.String()
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) >= 4 && fs[0] == want && fs[2] != "0x0" {
			if hw, err := net.ParseMAC(fs[3]); err == nil && hw.String() != "00:00:00:00:00:00" {
				return hw, true
			}
		}
	}
	return nil, false
}
