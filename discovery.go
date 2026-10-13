package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// DiscoveryItem is a generic row in a discovery result window.
type DiscoveryItem struct {
	Cols []string
}

// ---------- UPnP / SSDP ----------

type upnpDesc struct {
	Device struct {
		DeviceType   string `xml:"deviceType"`
		FriendlyName string `xml:"friendlyName"`
		Manufacturer string `xml:"manufacturer"`
		ModelName    string `xml:"modelName"`
		ModelNumber  string `xml:"modelNumber"`
		Presentation string `xml:"presentationURL"`
	} `xml:"device"`
}

func discoverUPnP(wait time.Duration) ([]DiscoveryItem, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	for _, st := range []string{"ssdp:all", "upnp:rootdevice"} {
		msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: " + st + "\r\n\r\n"
		conn.WriteToUDP([]byte(msg), dst)
	}
	type rec struct {
		ip, loc, server, st, usn string
	}
	byLoc := map[string]*rec{}
	deadline := time.Now().Add(wait)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		hdr := map[string]string{}
		for _, l := range strings.Split(string(buf[:n]), "\r\n") {
			if i := strings.Index(l, ":"); i > 0 {
				hdr[strings.ToUpper(strings.TrimSpace(l[:i]))] = strings.TrimSpace(l[i+1:])
			}
		}
		loc := hdr["LOCATION"]
		key := loc
		if key == "" {
			key = from.IP.String()
		}
		if _, ok := byLoc[key]; !ok {
			byLoc[key] = &rec{ip: from.IP.String(), loc: loc, server: hdr["SERVER"], st: hdr["ST"], usn: hdr["USN"]}
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var items []DiscoveryItem
	client := &http.Client{Timeout: 3 * time.Second}
	for _, r := range byLoc {
		wg.Add(1)
		go func(r *rec) {
			defer wg.Done()
			var d upnpDesc
			if r.loc != "" {
				if resp, err := client.Get(r.loc); err == nil {
					b, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
					resp.Body.Close()
					xml.Unmarshal(b, &d)
				}
			}
			dt := d.Device.DeviceType
			if i := strings.LastIndex(strings.TrimSuffix(dt, ":1"), ":"); i >= 0 {
				dt = strings.TrimSuffix(dt[i+1:], ":1")
			}
			model := strings.TrimSpace(d.Device.ModelName + " " + d.Device.ModelNumber)
			mu.Lock()
			items = append(items, DiscoveryItem{Cols: []string{r.ip, d.Device.FriendlyName, d.Device.Manufacturer, model, dt, r.server, r.loc}})
			mu.Unlock()
		}(r)
	}
	wg.Wait()
	sortItemsByIP(items)
	return items, nil
}

func sortItemsByIP(items []DiscoveryItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := net.ParseIP(items[i].Cols[0]), net.ParseIP(items[j].Cols[0])
		if a == nil || b == nil || a.To4() == nil || b.To4() == nil {
			return items[i].Cols[0] < items[j].Cols[0]
		}
		if ipToU32(a) != ipToU32(b) {
			return ipToU32(a) < ipToU32(b)
		}
		return strings.Join(items[i].Cols, "|") < strings.Join(items[j].Cols, "|")
	})
}

// ---------- mDNS / DNS-SD ----------

func discoverMDNS(wait time.Duration) ([]DiscoveryItem, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

	query := func(names []string, t dnsmessage.Type) {
		b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: 0})
		b.StartQuestions()
		for _, n := range names {
			nm, err := dnsmessage.NewName(n)
			if err != nil {
				continue
			}
			b.Question(dnsmessage.Question{Name: nm, Type: t, Class: dnsmessage.ClassINET | 0x8000})
		}
		if msg, err := b.Finish(); err == nil {
			conn.WriteToUDP(msg, dst)
		}
	}
	common := []string{"_services._dns-sd._udp.local.", "_http._tcp.local.", "_https._tcp.local.", "_workstation._tcp.local.",
		"_smb._tcp.local.", "_afpovertcp._tcp.local.", "_ipp._tcp.local.", "_printer._tcp.local.", "_pdl-datastream._tcp.local.",
		"_googlecast._tcp.local.", "_airplay._tcp.local.", "_raop._tcp.local.", "_spotify-connect._tcp.local.",
		"_hap._tcp.local.", "_homekit._tcp.local.", "_home-assistant._tcp.local.", "_esphomelib._tcp.local.",
		"_ssh._tcp.local.", "_sftp-ssh._tcp.local.", "_device-info._tcp.local.", "_sonos._tcp.local.", "_mqtt._tcp.local.",
		"_arduino._tcp.local.", "_shelly._tcp.local.", "_companion-link._tcp.local.", "_scanner._tcp.local.", "_uscan._tcp.local."}
	query(common, dnsmessage.TypePTR)

	type inst struct {
		name, service, host, port, txt string
		ips                            map[string]bool
	}
	instances := map[string]*inst{}
	hostIPs := map[string]map[string]bool{}
	seenTypes := map[string]bool{}
	for _, c := range common {
		seenTypes[c] = true
	}
	deadline := time.Now().Add(wait)
	buf := make([]byte, 9000)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		var p dnsmessage.Parser
		if _, err := p.Start(buf[:n]); err != nil {
			continue
		}
		p.SkipAllQuestions()
		var rrs []dnsmessage.Resource
		for {
			r, err := p.Answer()
			if err != nil {
				break
			}
			rrs = append(rrs, r)
		}
		p.SkipAllAuthorities()
		for {
			r, err := p.Additional()
			if err != nil {
				break
			}
			rrs = append(rrs, r)
		}
		var newTypes []string
		for _, r := range rrs {
			name := r.Header.Name.String()
			switch body := r.Body.(type) {
			case *dnsmessage.PTRResource:
				target := body.PTR.String()
				if name == "_services._dns-sd._udp.local." {
					if !seenTypes[target] {
						seenTypes[target] = true
						newTypes = append(newTypes, target)
					}
					continue
				}
				if _, ok := instances[target]; !ok {
					instances[target] = &inst{name: target, service: name, ips: map[string]bool{}}
				}
				instances[target].ips[from.IP.String()] = true
			case *dnsmessage.SRVResource:
				in := instances[name]
				if in == nil {
					in = &inst{name: name, ips: map[string]bool{}}
					instances[name] = in
				}
				in.host = body.Target.String()
				in.port = fmt.Sprint(body.Port)
				in.ips[from.IP.String()] = true
			case *dnsmessage.TXTResource:
				if in := instances[name]; in != nil {
					in.txt = strings.Join(body.TXT, "; ")
				}
			case *dnsmessage.AResource:
				ip := net.IP(body.A[:]).String()
				if hostIPs[name] == nil {
					hostIPs[name] = map[string]bool{}
				}
				hostIPs[name][ip] = true
			}
		}
		if len(newTypes) > 0 {
			query(newTypes, dnsmessage.TypePTR)
		}
	}
	var items []DiscoveryItem
	for _, in := range instances {
		ips := map[string]bool{}
		for ip := range in.ips {
			ips[ip] = true
		}
		for ip := range hostIPs[in.host] {
			ips = map[string]bool{ip: true}
		}
		ip := ""
		for k := range ips {
			ip = k
			break
		}
		svc := in.service
		instName := strings.TrimSuffix(in.name, "."+svc)
		instName = strings.ReplaceAll(instName, `\032`, " ")
		items = append(items, DiscoveryItem{Cols: []string{ip, instName, strings.TrimSuffix(svc, ".local."), strings.TrimSuffix(in.host, "."), in.port, in.txt}})
	}
	sortItemsByIP(items)
	return items, nil
}

// ---------- DHCP discovery ----------

func discoverDHCP(wait time.Duration) ([]DiscoveryItem, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 68})
	if err != nil {
		return nil, fmt.Errorf(T("cannot bind UDP port 68 (%v).\nRun the program as administrator or stop software that uses this port"), err)
	}
	defer conn.Close()
	mac := net.HardwareAddr{0x02, byte(rand.Intn(256)), byte(rand.Intn(256)), byte(rand.Intn(256)), byte(rand.Intn(256)), byte(rand.Intn(256))}
	if ifs, _ := net.Interfaces(); len(ifs) > 0 {
		for _, i := range ifs {
			if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 && len(i.HardwareAddr) == 6 {
				mac = i.HardwareAddr
				break
			}
		}
	}
	xid := rand.Uint32()
	p := make([]byte, 240)
	p[0], p[1], p[2] = 1, 1, 6
	binary.BigEndian.PutUint32(p[4:], xid)
	binary.BigEndian.PutUint16(p[10:], 0x8000)
	copy(p[28:], mac)
	copy(p[236:], []byte{99, 130, 83, 99})
	p = append(p, 53, 1, 1) // DHCPDISCOVER
	p = append(p, 55, 6, 1, 3, 6, 15, 51, 54)
	p = append(p, 255)
	conn.WriteToUDP(p, &net.UDPAddr{IP: net.IPv4bcast, Port: 67})

	var items []DiscoveryItem
	deadline := time.Now().Add(wait)
	buf := make([]byte, 1500)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		b := buf[:n]
		if n < 240 || b[0] != 2 || binary.BigEndian.Uint32(b[4:]) != xid {
			continue
		}
		offered := net.IP(b[16:20]).String()
		opts := map[byte][]byte{}
		for i := 240; i < n; {
			c := b[i]
			if c == 255 {
				break
			}
			if c == 0 {
				i++
				continue
			}
			if i+1 >= n {
				break
			}
			l := int(b[i+1])
			if i+2+l > n {
				break
			}
			opts[c] = b[i+2 : i+2+l]
			i += 2 + l
		}
		ipList := func(v []byte) string {
			var s []string
			for i := 0; i+4 <= len(v); i += 4 {
				s = append(s, net.IP(v[i:i+4]).String())
			}
			return strings.Join(s, ", ")
		}
		server := from.IP.String()
		if v, ok := opts[54]; ok {
			server = ipList(v)
		}
		lease := ""
		if v, ok := opts[51]; ok && len(v) == 4 {
			lease = (time.Duration(binary.BigEndian.Uint32(v)) * time.Second).String()
		}
		items = append(items, DiscoveryItem{Cols: []string{server, offered, ipList(opts[1]), ipList(opts[3]), ipList(opts[6]), string(opts[15]), lease}})
	}
	return items, nil
}

// ---------- External IP ----------

func externalIP() (string, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	var lastErr error
	for _, u := range []string{"https://api.ipify.org", "https://ifconfig.me/ip", "https://icanhazip.com"} {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
		req.Header.Set("User-Agent", "NetzScan")
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		cancel()
		ip := string(bytes.TrimSpace(b))
		if net.ParseIP(ip) != nil {
			return ip, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no answer")
	}
	return "", lastErr
}
