package main

import "errors"

// PingResult describes one ICMP echo.
type PingResult struct {
	OK  bool
	RTT uint32
	TTL uint8
}

var errNoIcmp = errors.New("ICMP not available")

// Share is an SMB share found on a host.
type Share struct {
	Name   string `xml:"name,attr" json:"name"`
	Type   uint32 `xml:"type,attr" json:"type"`
	Remark string `xml:"remark,attr,omitempty" json:"remark,omitempty"`
}

const (
	stypeDisk    = 0
	stypePrinter = 1
	stypeDevice  = 2
	stypeIPC     = 3
	stypeSpecial = 0x80000000
)

func (s Share) Kind() string {
	switch {
	case s.Type&0xff == stypePrinter:
		return T("Shared printer")
	case s.Type&0xff == stypeIPC:
		return "IPC"
	case s.Type&stypeSpecial != 0:
		return T("Administrative share")
	case s.Type&0xff == stypeDevice:
		return T("Device")
	}
	return T("Standard share")
}

func (s Share) IsAdmin() bool { return s.Type&stypeSpecial != 0 }
