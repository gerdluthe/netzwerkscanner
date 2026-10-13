package main

import (
	"encoding/binary"
	"net"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	modNetapi32 = windows.NewLazySystemDLL("netapi32.dll")
	modShell32  = windows.NewLazySystemDLL("shell32.dll")

	procIcmpCreateFile  = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpCloseHandle = modIphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho    = modIphlpapi.NewProc("IcmpSendEcho")
	procSendARP         = modIphlpapi.NewProc("SendARP")

	procNetShareEnum     = modNetapi32.NewProc("NetShareEnum")
	procNetApiBufferFree = modNetapi32.NewProc("NetApiBufferFree")

	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

type ipOptionInformation struct {
	Ttl         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

func ip4ToAddr(ip net.IP) uint32 {
	return binary.LittleEndian.Uint32(ip.To4())
}

// icmpPing sends a single ICMP echo request using the Windows ICMP API
// (works without administrator rights).
func icmpPing(ip net.IP, timeoutMs uint32) (PingResult, error) {
	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == uintptr(windows.InvalidHandle) {
		return PingResult{}, errNoIcmp
	}
	defer procIcmpCloseHandle.Call(h)

	payload := []byte("NetzScanPingData32bytes.........")
	replySize := uint32(unsafe.Sizeof(icmpEchoReply{})) + uint32(len(payload)) + 8 + 64
	buf := make([]byte, replySize)
	opts := ipOptionInformation{Ttl: 128}
	n, _, _ := procIcmpSendEcho.Call(h, uintptr(ip4ToAddr(ip)),
		uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)),
		uintptr(unsafe.Pointer(&opts)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(replySize), uintptr(timeoutMs))
	if n == 0 {
		return PingResult{}, nil
	}
	rep := (*icmpEchoReply)(unsafe.Pointer(&buf[0]))
	if rep.Status != 0 || rep.Address != ip4ToAddr(ip) {
		return PingResult{}, nil
	}
	return PingResult{OK: true, RTT: rep.RoundTripTime, TTL: rep.Options.Ttl}, nil
}

// arpLookup resolves the MAC address for an IPv4 address on the local subnet.
func arpLookup(ip net.IP) (net.HardwareAddr, bool) {
	var mac [8]byte
	l := uint32(8)
	r, _, _ := procSendARP.Call(uintptr(ip4ToAddr(ip)), 0, uintptr(unsafe.Pointer(&mac[0])), uintptr(unsafe.Pointer(&l)))
	if r != 0 || l < 6 {
		return nil, false
	}
	hw := net.HardwareAddr(append([]byte{}, mac[:6]...))
	allZero := true
	for _, b := range hw {
		if b != 0 {
			allZero = false
		}
	}
	if allZero {
		return nil, false
	}
	return hw, true
}

type shareInfo1 struct {
	Netname *uint16
	Type    uint32
	Remark  *uint16
}

var shareMu sync.Mutex

// enumShares lists the SMB shares of a host via NetShareEnum.
func enumShares(host string) ([]Share, error) {
	srv, _ := syscall.UTF16PtrFromString(`\\` + host)
	var buf uintptr
	var read, total, resume uint32
	r, _, _ := procNetShareEnum.Call(uintptr(unsafe.Pointer(srv)), 1, uintptr(unsafe.Pointer(&buf)),
		0xFFFFFFFF, uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume)))
	if buf != 0 {
		defer procNetApiBufferFree.Call(buf)
	}
	if r != 0 && r != 234 {
		return nil, syscall.Errno(r)
	}
	var out []Share
	sz := unsafe.Sizeof(shareInfo1{})
	for i := uint32(0); i < read; i++ {
		si := (*shareInfo1)(unsafe.Pointer(buf + uintptr(i)*sz))
		sh := Share{Name: windows.UTF16PtrToString(si.Netname), Type: si.Type, Remark: windows.UTF16PtrToString(si.Remark)}
		if sh.Type&0xff == stypeIPC {
			continue
		}
		out = append(out, sh)
	}
	return out, nil
}

func shellOpen(target string, args string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	t, _ := syscall.UTF16PtrFromString(target)
	var a *uint16
	if args != "" {
		a, _ = syscall.UTF16PtrFromString(args)
	}
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(a)), 0, 1)
}

func sharesSupported() bool { return true }
