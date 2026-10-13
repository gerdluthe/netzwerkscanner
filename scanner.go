package main

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Device is one scanned host.
type Device struct {
	IP          net.IP
	Alive       bool
	RTT         int // ms, -1 unknown
	TTL         int
	Method      string
	MAC         net.HardwareAddr
	Vendor      string
	HostName    string
	NetBIOS     string
	Workgroup   string
	User        string
	Ports       []int
	HTTPServer  string
	HTTPTitle   string
	Shares      []Share
	DeviceType  string
	LastScan    time.Time
	New         bool
	DupMAC      bool
	Expanded    bool
	Highlighted bool
}

func (d *Device) Key() string { return d.IP.String() }

// ---------- target parsing ----------

func ipToU32(ip net.IP) uint32 { return binary.BigEndian.Uint32(ip.To4()) }
func u32ToIP(v uint32) net.IP {
	b := make(net.IP, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

const maxTargets = 1 << 20

// parseRangeSpec parses "a.b.c.d", "a.b.c.d-e.f.g.h", "a.b.c.d-h", "a.b.c.d/nn" or a host name.
func parseRangeSpec(spec string) ([][2]uint32, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	if strings.Contains(spec, "/") {
		_, n, err := net.ParseCIDR(spec)
		if err != nil || n.IP.To4() == nil {
			return nil, fmt.Errorf(T("invalid CIDR: %s"), spec)
		}
		ones, bits := n.Mask.Size()
		start := ipToU32(n.IP)
		end := start | (uint32(1)<<(uint(bits-ones)) - 1)
		if ones <= 30 {
			start++
			end--
		}
		return [][2]uint32{{start, end}}, nil
	}
	if i := strings.Index(spec, "-"); i > 0 {
		a := net.ParseIP(strings.TrimSpace(spec[:i])).To4()
		bs := strings.TrimSpace(spec[i+1:])
		if a == nil {
			return nil, fmt.Errorf(T("invalid range: %s"), spec)
		}
		b := net.ParseIP(bs).To4()
		if b == nil {
			if n, err := strconv.Atoi(bs); err == nil && n >= 0 && n <= 255 {
				b = net.IPv4(a[0], a[1], a[2], byte(n)).To4()
			} else {
				return nil, fmt.Errorf(T("invalid range: %s"), spec)
			}
		}
		s, e := ipToU32(a), ipToU32(b)
		if s > e {
			return nil, fmt.Errorf(T("invalid IP address range: %s"), spec)
		}
		return [][2]uint32{{s, e}}, nil
	}
	if ip := net.ParseIP(spec).To4(); ip != nil {
		v := ipToU32(ip)
		return [][2]uint32{{v, v}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", spec)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf(T("cannot resolve host name: %s"), spec)
	}
	var out [][2]uint32
	for _, a := range addrs {
		v := ipToU32(a.To4())
		out = append(out, [2]uint32{v, v})
	}
	return out, nil
}

func buildTargets(from, to string, extra []string, ignored []string, random bool) ([]net.IP, error) {
	var ranges [][2]uint32
	if from != "" && from != "0.0.0.0" {
		a := net.ParseIP(from).To4()
		b := net.ParseIP(to).To4()
		if a == nil {
			return nil, fmt.Errorf(T("invalid IP address: %s"), from)
		}
		if b == nil || to == "0.0.0.0" {
			b = a
		}
		if ipToU32(a) > ipToU32(b) {
			return nil, fmt.Errorf(T("invalid IP address range: %s - %s"), from, to)
		}
		ranges = append(ranges, [2]uint32{ipToU32(a), ipToU32(b)})
	}
	for _, e := range extra {
		r, err := parseRangeSpec(e)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r...)
	}
	if len(ranges) == 0 {
		return nil, errors.New(T("please enter an IP address range to scan"))
	}
	ign := map[uint32]bool{}
	for _, s := range ignored {
		if r, err := parseRangeSpec(s); err == nil {
			for _, rr := range r {
				for v := rr[0]; v <= rr[1] && v-rr[0] < 65536; v++ {
					ign[v] = true
					if v == 0xffffffff {
						break
					}
				}
			}
		}
	}
	seen := map[uint32]bool{}
	var out []net.IP
	for _, r := range ranges {
		for v := r[0]; ; v++ {
			if !seen[v] && !ign[v] {
				seen[v] = true
				out = append(out, u32ToIP(v))
				if len(out) > maxTargets {
					return nil, fmt.Errorf(T("the range is too large (max. %d addresses)"), maxTargets)
				}
			}
			if v == r[1] {
				break
			}
		}
	}
	if random {
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	return out, nil
}

func parsePorts(s string) []int {
	set := map[int]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if i := strings.Index(part, "-"); i > 0 {
			a, e1 := strconv.Atoi(part[:i])
			b, e2 := strconv.Atoi(part[i+1:])
			if e1 == nil && e2 == nil {
				for p := a; p <= b && p <= 65535; p++ {
					if p > 0 {
						set[p] = true
					}
				}
			}
		} else if p, err := strconv.Atoi(part); err == nil && p > 0 && p <= 65535 {
			set[p] = true
		}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// ---------- local network info ----------

type LocalNet struct {
	Name  string
	IP    net.IP
	Net   *net.IPNet
	First net.IP
	Last  net.IP
}

func localNetworks() []LocalNet {
	var out []LocalNet
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			ones, bits := ipn.Mask.Size()
			if bits != 32 {
				continue
			}
			base := ipToU32(ipn.IP.To4()) & binary.BigEndian.Uint32(ipn.Mask)
			last := base | (uint32(1)<<(uint(32-ones)) - 1)
			first := base
			if ones <= 30 {
				first++
				last--
			}
			out = append(out, LocalNet{Name: ifc.Name, IP: ipn.IP.To4(), Net: ipn, First: u32ToIP(first), Last: u32ToIP(last)})
		}
	}
	if len(out) == 0 {
		// Fallback (e.g. Android 11+, where interface enumeration is restricted):
		// ask the routing table which local address would be used and assume a /24.
		if c, err := net.Dial("udp4", "192.0.2.1:9"); err == nil {
			if la, ok := c.LocalAddr().(*net.UDPAddr); ok && la.IP.To4() != nil && !la.IP.IsLoopback() {
				ip := la.IP.To4()
				_, n, _ := net.ParseCIDR(fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2]))
				out = append(out, LocalNet{Name: "default", IP: ip, Net: n, First: net.IPv4(ip[0], ip[1], ip[2], 1).To4(), Last: net.IPv4(ip[0], ip[1], ip[2], 254).To4()})
			}
			c.Close()
		}
	}
	return out
}

func inLocalNets(ip net.IP, nets []LocalNet) bool {
	for _, n := range nets {
		if n.Net.Contains(ip) {
			return true
		}
	}
	return false
}

// ---------- scanner ----------

type Scanner struct {
	cfg     *Settings
	targets []net.IP
	ports   []int
	nets    []LocalNet

	ctx    context.Context
	cancel context.CancelFunc

	pauseMu sync.Mutex
	paused  bool
	pauseCh chan struct{}

	Total   int64
	Done    int64
	Active  int64
	Found   int64
	Started time.Time

	Results chan *Device
	wg      sync.WaitGroup
}

func NewScanner(cfg *Settings, targets []net.IP) *Scanner {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scanner{cfg: cfg, targets: targets, ctx: ctx, cancel: cancel, Results: make(chan *Device, 4096),
		nets: localNetworks(), pauseCh: make(chan struct{})}
	close(s.pauseCh)
	if cfg.CheckPorts {
		s.ports = parsePorts(cfg.Ports)
	}
	s.Total = int64(len(targets))
	return s
}

func (s *Scanner) Start() {
	s.Started = time.Now()
	jobs := make(chan net.IP)
	n := s.cfg.MaxThreads
	if n > len(s.targets) {
		n = len(s.targets)
	}
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for ip := range jobs {
				s.waitPause()
				if s.ctx.Err() != nil {
					atomic.AddInt64(&s.Done, 1)
					continue
				}
				atomic.AddInt64(&s.Active, 1)
				d := s.scanHost(ip)
				atomic.AddInt64(&s.Active, -1)
				atomic.AddInt64(&s.Done, 1)
				if d != nil {
					if d.Alive {
						atomic.AddInt64(&s.Found, 1)
					}
					select {
					case s.Results <- d:
					case <-s.ctx.Done():
					}
				}
			}
		}()
	}
	go func() {
		for _, ip := range s.targets {
			if s.ctx.Err() != nil {
				break
			}
			jobs <- ip
		}
		close(jobs)
		s.wg.Wait()
		close(s.Results)
	}()
}

func (s *Scanner) Stop() { s.cancel(); s.SetPaused(false) }

func (s *Scanner) SetPaused(p bool) {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if p == s.paused {
		return
	}
	s.paused = p
	if p {
		s.pauseCh = make(chan struct{})
	} else {
		close(s.pauseCh)
	}
}

func (s *Scanner) Paused() bool {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.paused
}

func (s *Scanner) waitPause() {
	s.pauseMu.Lock()
	ch := s.pauseCh
	s.pauseMu.Unlock()
	select {
	case <-ch:
	case <-s.ctx.Done():
	}
}

func (s *Scanner) timeout() time.Duration {
	return time.Duration(s.cfg.PingTimeout) * time.Millisecond
}

func (s *Scanner) portTimeout() time.Duration {
	t := s.timeout()
	if t < 300*time.Millisecond {
		t = 300 * time.Millisecond
	}
	if t > 2*time.Second {
		t = 2 * time.Second
	}
	return t
}

func (s *Scanner) scanHost(ip net.IP) *Device {
	return ScanSingle(s.ctx, s.cfg, ip, s.ports, s.nets)
}

// ScanSingle runs the full analysis for one address.
func ScanSingle(ctx context.Context, cfg *Settings, ip net.IP, ports []int, nets []LocalNet) *Device {
	d := &Device{IP: ip, RTT: -1, LastScan: time.Now()}
	to := time.Duration(cfg.PingTimeout) * time.Millisecond
	local := inLocalNets(ip, nets)

	if cfg.PingMethod != 1 {
		noICMP := false
		for a := 0; a < cfg.Attempts && !d.Alive && ctx.Err() == nil; a++ {
			r, err := icmpPing(ip, uint32(cfg.PingTimeout))
			if err == errNoIcmp {
				noICMP = true
				break
			}
			if err == nil && r.OK {
				d.Alive, d.RTT, d.TTL, d.Method = true, int(r.RTT), int(r.TTL), "ICMP"
			}
		}
		// Without ICMP rights (some Android/Linux systems) a TCP probe is used:
		// an accepted or refused connection both prove that the host exists.
		if noICMP && !d.Alive && ctx.Err() == nil {
			if rtt, ok := tcpProbe(ip, to); ok {
				d.Alive, d.RTT, d.Method = true, rtt, "TCP"
			}
		}
	}
	if cfg.PingMethod != 0 && local && ctx.Err() == nil {
		if mac, ok := arpLookup(ip); ok {
			d.MAC = mac
			if !d.Alive {
				d.Alive, d.Method = true, "ARP"
			}
		}
	}
	if !d.Alive && !cfg.AlwaysAnalyse {
		if cfg.DisplayInactive {
			return d
		}
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}

	if cfg.ResolveMAC && d.MAC == nil && local {
		if mac, ok := arpLookup(ip); ok {
			d.MAC = mac
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// NetBIOS
	if (cfg.ResolveHostNames && cfg.HostNameMethod != 1) || cfg.LookupWorkgroup || cfg.LookupUser {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nbTo := to
			if nbTo > 1500*time.Millisecond {
				nbTo = 1500 * time.Millisecond
			}
			if nbTo < 400*time.Millisecond {
				nbTo = 400 * time.Millisecond
			}
			if nb, ok := netbiosQuery(ip, nbTo); ok {
				mu.Lock()
				d.NetBIOS, d.Workgroup, d.User = nb.Name, nb.Workgroup, nb.User
				if d.MAC == nil && nb.MAC != nil && cfg.ResolveMAC {
					d.MAC = nb.MAC
				}
				mu.Unlock()
			}
		}()
	}
	// DNS
	if cfg.ResolveHostNames && cfg.HostNameMethod != 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if names, err := net.DefaultResolver.LookupAddr(c, ip.String()); err == nil && len(names) > 0 {
				mu.Lock()
				d.HostName = strings.TrimSuffix(names[0], ".")
				mu.Unlock()
			}
		}()
	}
	// Ports
	if len(ports) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			open := scanPorts(ctx, ip, ports, portTimeoutFor(to))
			mu.Lock()
			d.Ports = open
			mu.Unlock()
		}()
	}
	wg.Wait()

	if d.HostName == "" && d.NetBIOS != "" && cfg.ResolveHostNames {
		d.HostName = d.NetBIOS
	}
	if !d.Alive && (len(d.Ports) > 0 || d.NetBIOS != "") {
		d.Alive = true
		if len(d.Ports) > 0 {
			d.Method = "TCP"
		} else {
			d.Method = "NetBIOS"
		}
	}

	// Shares
	if cfg.EnumShares && ctx.Err() == nil {
		if hasPort(d.Ports, 445) || hasPort(d.Ports, 139) || tcpOpen(ip, 445, portTimeoutFor(to)) || tcpOpen(ip, 139, portTimeoutFor(to)) {
			if sh, err := enumSharesTimeout(ip.String(), 8*time.Second); err == nil {
				for _, x := range sh {
					if !cfg.ShowAdminShares && x.IsAdmin() {
						continue
					}
					if !cfg.ShowPrinterShares && x.Type&0xff == stypePrinter {
						continue
					}
					d.Shares = append(d.Shares, x)
				}
			}
		}
	}

	// HTTP banner
	if cfg.GrabHTTP && ctx.Err() == nil {
		grabHTTP(d, portTimeoutFor(to))
	}

	if cfg.LookupVendor && d.MAC != nil {
		d.Vendor = vendorOf(d.MAC)
	}
	if cfg.DeviceTypeGuess {
		d.DeviceType = guessDeviceType(d, nets)
	}
	return d
}

var probePorts = []int{80, 443, 22, 445, 139, 53, 8080, 62078, 7000, 8008, 1883, 554, 5000, 9100}

func tcpProbe(ip net.IP, to time.Duration) (int, bool) {
	if to < 400*time.Millisecond {
		to = 400 * time.Millisecond
	}
	type res struct {
		ok  bool
		rtt int
	}
	ch := make(chan res, len(probePorts))
	start := time.Now()
	for _, p := range probePorts {
		go func(p int) {
			c, err := net.DialTimeout("tcp4", net.JoinHostPort(ip.String(), strconv.Itoa(p)), to)
			if err == nil {
				c.Close()
				ch <- res{true, int(time.Since(start).Milliseconds())}
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "refused") {
				ch <- res{true, int(time.Since(start).Milliseconds())}
				return
			}
			ch <- res{}
		}(p)
	}
	for range probePorts {
		if r := <-ch; r.ok {
			return r.rtt, true
		}
	}
	return 0, false
}

func portTimeoutFor(t time.Duration) time.Duration {
	if t < 300*time.Millisecond {
		t = 300 * time.Millisecond
	}
	if t > 2*time.Second {
		t = 2 * time.Second
	}
	return t
}

func hasPort(ps []int, p int) bool {
	for _, x := range ps {
		if x == p {
			return true
		}
	}
	return false
}

func tcpOpen(ip net.IP, port int, to time.Duration) bool {
	c, err := net.DialTimeout("tcp4", net.JoinHostPort(ip.String(), strconv.Itoa(port)), to)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func scanPorts(ctx context.Context, ip net.IP, ports []int, to time.Duration) []int {
	sem := make(chan struct{}, 64)
	var mu sync.Mutex
	var open []int
	var wg sync.WaitGroup
	for _, p := range ports {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()
			if tcpOpen(ip, p, to) {
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	sort.Ints(open)
	return open
}

func enumSharesTimeout(host string, to time.Duration) ([]Share, error) {
	type res struct {
		s   []Share
		err error
	}
	ch := make(chan res, 1)
	go func() { s, err := enumShares(host); ch <- res{s, err} }()
	select {
	case r := <-ch:
		return r.s, r.err
	case <-time.After(to):
		return nil, fmt.Errorf("timeout")
	}
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func grabHTTP(d *Device, to time.Duration) {
	type cand struct {
		scheme string
		port   int
	}
	var cands []cand
	if len(d.Ports) > 0 {
		for _, p := range d.Ports {
			switch p {
			case 80, 81, 8000, 8008, 8080, 8081, 8123, 8888, 9000, 9090, 3000, 5000:
				cands = append(cands, cand{"http", p})
			case 443, 8443:
				cands = append(cands, cand{"https", p})
			}
		}
	} else {
		if tcpOpen(d.IP, 80, to) {
			cands = append(cands, cand{"http", 80})
		} else if tcpOpen(d.IP, 443, to) {
			cands = append(cands, cand{"https", 443})
		}
	}
	if len(cands) == 0 {
		return
	}
	c := cands[0]
	client := &http.Client{Timeout: 3 * time.Second,
		Transport: &http.Transport{
			// Only used to read the banner/title of devices on the local network,
			// which almost always use self-signed certificates.
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true, Proxy: nil},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 3 {
				return http.ErrUseLastResponse
			}
			return nil
		}}
	url := fmt.Sprintf("%s://%s:%d/", c.scheme, d.IP, c.port)
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	d.HTTPServer = resp.Header.Get("Server")
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if m := titleRe.FindSubmatch(body); m != nil {
		t := strings.Join(strings.Fields(string(m[1])), " ")
		if len(t) > 120 {
			t = t[:120]
		}
		d.HTTPTitle = t
	}
}

func guessDeviceType(d *Device, nets []LocalNet) string {
	v := strings.ToLower(d.Vendor)
	h := strings.ToLower(d.HostName + " " + d.NetBIOS + " " + d.HTTPTitle + " " + d.HTTPServer)
	has := func(ps ...int) bool {
		for _, p := range ps {
			if hasPort(d.Ports, p) {
				return true
			}
		}
		return false
	}
	any := func(s string, keys ...string) bool {
		for _, k := range keys {
			if strings.Contains(s, k) {
				return true
			}
		}
		return false
	}
	for _, n := range nets {
		if d.IP.Equal(n.IP) {
			return T("This computer")
		}
	}
	switch {
	case any(h, "fritz"):
		return T("Router (FRITZ!Box)")
	case any(v, "avm audiovisuelles", "avm gmbh"):
		return T("AVM FRITZ! device")
	case any(h, "home assistant") || has(8123):
		return T("Home Assistant")
	case any(v, "raspberry"):
		return T("Raspberry Pi")
	case any(v, "espressif"):
		return T("IoT device (ESP)")
	case any(v, "shelly", "allterco"):
		return T("Shelly IoT device")
	case any(v, "victron"):
		return T("Victron device")
	case any(v, "hikvision", "dahua", "axis comm", "reolink", "foscam", "amcrest", "ubiquiti") && has(554):
		return T("IP camera")
	case has(554) && !has(445):
		return T("IP camera / media")
	case has(9100, 631, 515) || any(h, "printer", "laserjet", "officejet"):
		return T("Printer")
	case any(v, "brother", "canon", "epson", "kyocera", "lexmark", "xerox", "ricoh"):
		return T("Printer")
	case any(v, "synology", "qnap", "western digital", "buffalo") || any(h, "synology", "qnap", "diskstation"):
		return T("NAS")
	case any(v, "sonos"):
		return T("Sonos speaker")
	case any(v, "amazon"):
		return T("Amazon device")
	case any(v, "google"):
		return T("Google device")
	case any(v, "apple"):
		if has(22, 445, 548) {
			return "Mac"
		}
		return T("Apple device")
	case any(v, "tp-link", "netgear", "mikrotik", "cisco", "juniper", "zyxel", "d-link", "aruba", "lancom", "huawei tech", "routerboard"):
		if has(53) || d.IP.To4()[3] == 1 {
			return T("Router / Gateway")
		}
		return T("Network device")
	case has(3389) || (has(135) && has(445)):
		return T("Windows computer")
	case any(v, "samsung", "lg electronics", "sony", "panasonic", "philips", "tcl", "hisense") && has(8001, 8008, 9080, 7000):
		return T("Smart TV")
	case any(v, "xiaomi", "oneplus", "oppo", "vivo", "motorola", "samsung") && len(d.Ports) == 0:
		return T("Mobile device")
	case any(v, "intel", "realtek", "dell", "lenovo", "micro-star", "giga-byte", "asustek", "hewlett", "microsoft", "fujitsu", "liteon", "azurewave"):
		return T("Computer")
	case has(53) && (has(80) || has(443)):
		return T("Router / Gateway")
	case has(1883, 8883):
		return T("MQTT broker")
	case has(502):
		return T("Modbus device")
	case has(22) && d.TTL > 0 && d.TTL <= 64:
		return T("Linux/Unix host")
	case d.TTL > 0 && d.TTL <= 64 && len(d.Ports) > 0:
		return T("Linux / embedded device")
	case d.TTL > 64 && d.TTL <= 128:
		return T("Windows device")
	case d.TTL > 128:
		return T("Network device")
	}
	return ""
}

// ---------- Wake-on-LAN ----------

func parseMAC(s string) (net.HardwareAddr, error) {
	s = strings.TrimSpace(s)
	clean := strings.NewReplacer("-", "", ":", "", ".", "", " ", "").Replace(s)
	if len(clean) != 12 {
		return nil, fmt.Errorf(T("invalid MAC address: %s"), s)
	}
	b := make([]byte, 6)
	for i := 0; i < 6; i++ {
		v, err := strconv.ParseUint(clean[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf(T("invalid MAC address: %s"), s)
		}
		b[i] = byte(v)
	}
	return b, nil
}

func sendWOL(mac net.HardwareAddr, targets ...string) error {
	pkt := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xff)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, mac...)
	}
	dests := map[string]bool{"255.255.255.255": true}
	for _, t := range targets {
		if t != "" {
			dests[t] = true
		}
	}
	for _, n := range localNetworks() {
		b := ipToU32(n.IP) | ^binary.BigEndian.Uint32(n.Net.Mask)
		dests[u32ToIP(b).String()] = true
	}
	var lastErr error
	sent := 0
	for d := range dests {
		for _, port := range []string{"9", "7"} {
			c, err := net.Dial("udp4", net.JoinHostPort(d, port))
			if err != nil {
				lastErr = err
				continue
			}
			if _, err := c.Write(pkt); err == nil {
				sent++
			} else {
				lastErr = err
			}
			c.Close()
		}
	}
	if sent == 0 {
		return lastErr
	}
	return nil
}

func formatMAC(mac net.HardwareAddr, format int) string {
	if mac == nil {
		return ""
	}
	parts := make([]string, len(mac))
	for i, b := range mac {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	switch format {
	case 1:
		return strings.Join(parts, ":")
	case 2:
		return strings.ToLower(strings.Join(parts, ""))
	case 3:
		return strings.ToLower(strings.Join(parts, ":"))
	}
	return strings.Join(parts, "-")
}
