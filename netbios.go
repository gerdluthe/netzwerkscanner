package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// NetBIOSInfo is the result of an NBSTAT node status query.
type NetBIOSInfo struct {
	Name      string
	Workgroup string
	User      string
	MAC       net.HardwareAddr
}

func netbiosQuery(ip net.IP, timeout time.Duration) (*NetBIOSInfo, bool) {
	conn, err := net.DialTimeout("udp4", net.JoinHostPort(ip.String(), "137"), timeout)
	if err != nil {
		return nil, false
	}
	defer conn.Close()
	id := uint16(rand.Intn(0xffff))
	q := []byte{byte(id >> 8), byte(id), 0x00, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0x20}
	name := []byte("*\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	for _, c := range name {
		q = append(q, 'A'+(c>>4), 'A'+(c&0x0f))
	}
	q = append(q, 0x00, 0x00, 0x21, 0x00, 0x01)
	buf := make([]byte, 1024)
	for attempt := 0; attempt < 2; attempt++ {
		conn.SetDeadline(time.Now().Add(timeout))
		if _, err := conn.Write(q); err != nil {
			return nil, false
		}
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}
		return parseNBStat(buf[:n])
	}
	return nil, false
}

func parseNBStat(b []byte) (*NetBIOSInfo, bool) {
	// header(12) + name(34) + type(2)+class(2)+ttl(4)+rdlen(2) = 56
	if len(b) < 57 {
		return nil, false
	}
	p := 56
	num := int(b[p])
	p++
	info := &NetBIOSInfo{}
	for i := 0; i < num && p+18 <= len(b); i++ {
		nm := strings.TrimRight(string(b[p:p+15]), " \x00")
		suffix := b[p+15]
		flags := uint16(b[p+16])<<8 | uint16(b[p+17])
		group := flags&0x8000 != 0
		switch {
		case suffix == 0x00 && !group && info.Name == "":
			info.Name = nm
		case suffix == 0x00 && group && info.Workgroup == "":
			info.Workgroup = nm
		case suffix == 0x03 && !group && nm != info.Name && info.User == "":
			info.User = nm
		case suffix == 0x20 && info.Name == "":
			info.Name = nm
		}
		p += 18
	}
	if p+6 <= len(b) {
		mac := net.HardwareAddr(append([]byte{}, b[p:p+6]...))
		if !bytes.Equal(mac, make([]byte, 6)) {
			info.MAC = mac
		}
	}
	return info, info.Name != "" || info.Workgroup != ""
}

//go:embed assets/oui.txt.gz
var ouiGz []byte

var (
	ouiOnce sync.Once
	ouiMap  map[string]string
)

func vendorOf(mac net.HardwareAddr) string {
	if len(mac) < 3 {
		return ""
	}
	ouiOnce.Do(func() {
		ouiMap = make(map[string]string, 60000)
		zr, err := gzip.NewReader(bytes.NewReader(ouiGz))
		if err != nil {
			return
		}
		sc := bufio.NewScanner(zr)
		for sc.Scan() {
			l := sc.Text()
			if i := strings.IndexByte(l, '\t'); i == 6 {
				ouiMap[l[:6]] = l[7:]
			}
		}
	})
	if mac[0]&0x02 != 0 {
		return "(locally administered)"
	}
	key := strings.ToUpper(strings.ReplaceAll(mac[:3].String(), ":", ""))
	return ouiMap[key]
}

var serviceNames = map[int]string{
	20: "ftp-data", 21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "dns", 67: "dhcp", 69: "tftp", 80: "http",
	81: "http-alt", 88: "kerberos", 110: "pop3", 111: "rpcbind", 119: "nntp", 123: "ntp", 135: "msrpc", 137: "netbios-ns",
	139: "netbios-ssn", 143: "imap", 161: "snmp", 389: "ldap", 443: "https", 445: "microsoft-ds", 465: "smtps",
	502: "modbus", 515: "printer", 548: "afp", 554: "rtsp", 587: "submission", 631: "ipp", 636: "ldaps", 853: "dns-tls",
	873: "rsync", 993: "imaps", 995: "pop3s", 1080: "socks", 1433: "ms-sql", 1521: "oracle", 1723: "pptp", 1883: "mqtt",
	1900: "upnp", 2049: "nfs", 2375: "docker", 3000: "http-dev", 3306: "mysql", 3389: "ms-rdp", 3483: "slimproto",
	4840: "opc-ua", 5000: "upnp/http", 5060: "sip", 5353: "mdns", 5432: "postgresql", 5900: "vnc", 5985: "winrm",
	6379: "redis", 6443: "kubernetes", 7547: "tr-069", 8000: "http-alt", 8008: "http-alt", 8080: "http-proxy",
	8081: "http-alt", 8123: "home-assistant", 8443: "https-alt", 8883: "mqtts", 8888: "http-alt", 9000: "http-alt",
	9100: "jetdirect", 9090: "http-alt", 10000: "webmin", 27017: "mongodb", 32400: "plex", 49152: "upnp",
}

func serviceName(port int) string {
	if s, ok := serviceNames[port]; ok {
		return s
	}
	return ""
}
