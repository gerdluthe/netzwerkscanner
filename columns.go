package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ColumnDef struct {
	ID      string
	Title   string
	Width   int
	Right   bool
	Default bool
	Auto    func(c *Settings) bool
	Get     func(d *Device, c *Settings) string
	Num     func(d *Device) float64 // optional numeric sort key
}

func portsString(d *Device, c *Settings) string {
	s := make([]string, len(d.Ports))
	for i, p := range d.Ports {
		if c.ServiceNames {
			if n := serviceName(p); n != "" {
				s[i] = n
				continue
			}
		}
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ", ")
}

func friendlyName(d *Device, c *Settings) string {
	if c.FriendlyNames == nil {
		return ""
	}
	if d.MAC != nil {
		if n, ok := c.FriendlyNames[formatMAC(d.MAC, 0)]; ok {
			return n
		}
	}
	return c.FriendlyNames[d.IP.String()]
}

var allColumns = []*ColumnDef{
	{ID: "ip", Title: "IP address", Width: 130, Default: true, Get: func(d *Device, c *Settings) string { return d.IP.String() },
		Num: func(d *Device) float64 { return float64(ipToU32(d.IP)) }},
	{ID: "mac", Title: "MAC address", Width: 140, Default: true, Get: func(d *Device, c *Settings) string { return formatMAC(d.MAC, c.MACFormat) }},
	{ID: "rtt", Title: "Response Time", Width: 100, Default: true, Get: func(d *Device, c *Settings) string {
		if !d.Alive || d.RTT < 0 {
			return ""
		}
		return fmt.Sprintf("%d ms", d.RTT)
	}, Num: func(d *Device) float64 {
		if !d.Alive || d.RTT < 0 {
			return 1e9
		}
		return float64(d.RTT)
	}},
	{ID: "host", Title: "Host name", Width: 170, Default: true, Get: func(d *Device, c *Settings) string { return d.HostName }},
	{ID: "type", Title: "Device Type", Width: 150, Default: true, Get: func(d *Device, c *Settings) string { return d.DeviceType }},
	{ID: "vendor", Title: "MAC Vendor", Width: 190, Auto: func(c *Settings) bool { return c.LookupVendor }, Get: func(d *Device, c *Settings) string { return d.Vendor }},
	{ID: "ports", Title: "Open Ports", Width: 160, Auto: func(c *Settings) bool { return c.CheckPorts }, Get: portsString},
	{ID: "workgroup", Title: "Workgroup", Width: 110, Auto: func(c *Settings) bool { return c.LookupWorkgroup }, Get: func(d *Device, c *Settings) string { return d.Workgroup }},
	{ID: "user", Title: "Logged-on User", Width: 120, Auto: func(c *Settings) bool { return c.LookupUser }, Get: func(d *Device, c *Settings) string { return d.User }},
	{ID: "ttl", Title: "TTL", Width: 50, Right: true, Auto: func(c *Settings) bool { return c.DisplayTTL }, Get: func(d *Device, c *Settings) string {
		if d.TTL <= 0 {
			return ""
		}
		return strconv.Itoa(d.TTL)
	}, Num: func(d *Device) float64 { return float64(d.TTL) }},
	{ID: "method", Title: "Ping Method", Width: 90, Auto: func(c *Settings) bool { return c.ShowPingMethod }, Get: func(d *Device, c *Settings) string { return d.Method }},
	{ID: "httpserver", Title: "HTTP Server", Width: 130, Auto: func(c *Settings) bool { return c.GrabHTTP }, Get: func(d *Device, c *Settings) string { return d.HTTPServer }},
	{ID: "httptitle", Title: "Web Page Title", Width: 180, Auto: func(c *Settings) bool { return c.GrabHTTP }, Get: func(d *Device, c *Settings) string { return d.HTTPTitle }},
	{ID: "comment", Title: "Comment", Width: 150, Auto: func(c *Settings) bool { return len(c.FriendlyNames) > 0 }, Get: friendlyName},
	{ID: "lastscan", Title: "Last Scan", Width: 130, Auto: func(c *Settings) bool { return c.ShowLastScan }, Get: func(d *Device, c *Settings) string {
		if d.LastScan.IsZero() {
			return ""
		}
		return d.LastScan.Format("2006-01-02 15:04:05")
	}, Num: func(d *Device) float64 { return float64(d.LastScan.Unix()) }},
}

func columnByID(id string) (int, *ColumnDef) {
	for i, c := range allColumns {
		if c.ID == id {
			return i, c
		}
	}
	return -1, nil
}

func (c *ColumnDef) Visible(cfg *Settings) bool {
	if cfg.HiddenColumns[c.ID] {
		return false
	}
	return c.Default || (c.Auto != nil && c.Auto(cfg))
}

func sortDevicesByIP(devs []*Device) {
	sort.SliceStable(devs, func(i, j int) bool { return ipToU32(devs[i].IP) < ipToU32(devs[j].IP) })
}
