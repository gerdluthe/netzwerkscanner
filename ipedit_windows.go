package main

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"netzscan/third_party/walk"
)

const (
	ipmClearAddress = win.WM_USER + 100
	ipmSetAddress   = win.WM_USER + 101
	ipmGetAddress   = win.WM_USER + 102
	iccInternet     = 0x00000800
)

// IPEdit is the native Windows IP address control (SysIPAddress32) with the
// classic segmented "0 . 0 . 0 . 0" input. It lives inside a fixed-size host
// composite, because the control lays out its four fields only on creation.
type IPEdit struct {
	host *walk.Composite
	hwnd win.HWND
}

func NewIPEdit(host *walk.Composite, fontFrom walk.Window) (*IPEdit, error) {
	var icc win.INITCOMMONCONTROLSEX
	icc.DwSize = uint32(unsafe.Sizeof(icc))
	icc.DwICC = iccInternet
	win.InitCommonControlsEx(&icc)

	cls, _ := syscall.UTF16PtrFromString("SysIPAddress32")
	dpi := host.DPI()
	sz := walk.Size{Width: 140 * dpi / 96, Height: 23 * dpi / 96}
	hwnd := win.CreateWindowEx(win.WS_EX_CLIENTEDGE, cls, nil,
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP,
		0, 0, int32(sz.Width), int32(sz.Height), host.Handle(), 0, 0, nil)
	if hwnd == 0 {
		return nil, fmt.Errorf("cannot create IP address control")
	}
	if fontFrom != nil {
		f := win.SendMessage(fontFrom.Handle(), win.WM_GETFONT, 0, 0)
		if f != 0 {
			win.SendMessage(hwnd, win.WM_SETFONT, f, 1)
		}
	}
	ie := &IPEdit{host: host, hwnd: hwnd}
	return ie, nil
}

func (ie *IPEdit) IP() net.IP {
	var v uint32
	win.SendMessage(ie.hwnd, ipmGetAddress, 0, uintptr(unsafe.Pointer(&v)))
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v)).To4()
}

func (ie *IPEdit) IPString() string { return ie.IP().String() }

func (ie *IPEdit) SetIP(ip net.IP) {
	ip4 := ip.To4()
	if ip4 == nil {
		win.SendMessage(ie.hwnd, ipmClearAddress, 0, 0)
		return
	}
	v := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
	win.SendMessage(ie.hwnd, ipmSetAddress, 0, uintptr(v))
}

func (ie *IPEdit) SetIPString(s string) error {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid IP address: %s", s)
	}
	ie.SetIP(ip)
	return nil
}

func (ie *IPEdit) SetFocus() { win.SetFocus(ie.hwnd) }
