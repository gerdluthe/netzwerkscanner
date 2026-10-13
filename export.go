package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type xmlDevice struct {
	IP         string  `xml:"ip,attr" json:"ip"`
	Alive      bool    `xml:"alive,attr" json:"alive"`
	RTT        int     `xml:"rtt,attr" json:"rtt"`
	TTL        int     `xml:"ttl,attr,omitempty" json:"ttl,omitempty"`
	Method     string  `xml:"method,attr,omitempty" json:"method,omitempty"`
	MAC        string  `xml:"mac,attr,omitempty" json:"mac,omitempty"`
	Vendor     string  `xml:"vendor,attr,omitempty" json:"vendor,omitempty"`
	HostName   string  `xml:"hostname,attr,omitempty" json:"hostname,omitempty"`
	NetBIOS    string  `xml:"netbios,attr,omitempty" json:"netbios,omitempty"`
	Workgroup  string  `xml:"workgroup,attr,omitempty" json:"workgroup,omitempty"`
	User       string  `xml:"user,attr,omitempty" json:"user,omitempty"`
	Ports      string  `xml:"ports,attr,omitempty" json:"ports,omitempty"`
	HTTPServer string  `xml:"httpserver,attr,omitempty" json:"httpServer,omitempty"`
	HTTPTitle  string  `xml:"httptitle,attr,omitempty" json:"httpTitle,omitempty"`
	DeviceType string  `xml:"type,attr,omitempty" json:"deviceType,omitempty"`
	LastScan   string  `xml:"lastscan,attr,omitempty" json:"lastScan,omitempty"`
	Shares     []Share `xml:"share" json:"shares,omitempty"`
}

type xmlResults struct {
	XMLName xml.Name    `xml:"netzscan" json:"-"`
	Version string      `xml:"version,attr" json:"version"`
	Created string      `xml:"created,attr" json:"created"`
	Range   string      `xml:"range,attr,omitempty" json:"range,omitempty"`
	Devices []xmlDevice `xml:"device" json:"devices"`
}

func toXML(d *Device) xmlDevice {
	ps := make([]string, len(d.Ports))
	for i, p := range d.Ports {
		ps[i] = strconv.Itoa(p)
	}
	x := xmlDevice{IP: d.IP.String(), Alive: d.Alive, RTT: d.RTT, TTL: d.TTL, Method: d.Method, MAC: formatMAC(d.MAC, 0),
		Vendor: d.Vendor, HostName: d.HostName, NetBIOS: d.NetBIOS, Workgroup: d.Workgroup, User: d.User,
		Ports: strings.Join(ps, ","), HTTPServer: d.HTTPServer, HTTPTitle: d.HTTPTitle, DeviceType: d.DeviceType, Shares: d.Shares}
	if !d.LastScan.IsZero() {
		x.LastScan = d.LastScan.Format(time.RFC3339)
	}
	return x
}

func fromXML(x xmlDevice) *Device {
	d := &Device{IP: net.ParseIP(x.IP).To4(), Alive: x.Alive, RTT: x.RTT, TTL: x.TTL, Method: x.Method, Vendor: x.Vendor,
		HostName: x.HostName, NetBIOS: x.NetBIOS, Workgroup: x.Workgroup, User: x.User, HTTPServer: x.HTTPServer,
		HTTPTitle: x.HTTPTitle, DeviceType: x.DeviceType, Shares: x.Shares}
	if x.MAC != "" {
		d.MAC, _ = parseMAC(x.MAC)
	}
	d.Ports = parsePorts(x.Ports)
	if t, err := time.Parse(time.RFC3339, x.LastScan); err == nil {
		d.LastScan = t
	}
	return d
}

func saveResults(path string, devs []*Device, cols []*ColumnDef, cfg *Settings, rangeDesc string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := saveResultsTo(f, strings.ToLower(filepath.Ext(path)), devs, cols, cfg, rangeDesc); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// saveResultsTo writes the results in the format given by ext (".xml", ".json", ".csv", ".html", ".txt").
func saveResultsTo(out io.Writer, ext string, devs []*Device, cols []*ColumnDef, cfg *Settings, rangeDesc string) error {
	switch ext {
	case ".xml", ".json":
		r := xmlResults{Version: appVersion, Created: time.Now().Format(time.RFC3339), Range: rangeDesc}
		for _, d := range devs {
			r.Devices = append(r.Devices, toXML(d))
		}
		var b []byte
		var err error
		if ext == ".xml" {
			b, err = xml.MarshalIndent(r, "", "  ")
			b = append([]byte(xml.Header), b...)
		} else {
			b, err = json.MarshalIndent(r, "", "  ")
		}
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	case ".csv":
		out.Write([]byte{0xEF, 0xBB, 0xBF})
		w := csv.NewWriter(out)
		if cfg.CSVDelim != "" {
			w.Comma = []rune(cfg.CSVDelim)[0]
		}
		var hdr []string
		for _, c := range cols {
			hdr = append(hdr, T(c.Title))
		}
		w.Write(hdr)
		for _, d := range devs {
			var row []string
			for _, c := range cols {
				row = append(row, c.Get(d, cfg))
			}
			w.Write(row)
			for _, s := range d.Shares {
				row := make([]string, len(cols))
				row[0] = `\\` + d.IP.String() + `\` + s.Name
				if len(cols) > 1 {
					row[1] = s.Kind()
				}
				if len(cols) > 2 {
					row[2] = s.Remark
				}
				w.Write(row)
			}
		}
		w.Flush()
		return w.Error()
	case ".html", ".htm":
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>NetzScan results</title><style>")
		sb.WriteString("body{font-family:Segoe UI,Arial,sans-serif;font-size:13px;color:#222;margin:20px}h1{font-size:18px}")
		sb.WriteString("table{border-collapse:collapse}th{background:#f0f0f0;text-align:left}th,td{border:1px solid #ccc;padding:4px 8px}")
		sb.WriteString("tr.off td{color:#999}tr.share td{color:#555;font-size:12px}td.sh{padding-left:24px}</style></head><body>")
		fmt.Fprintf(&sb, "<h1>Network scan results</h1><p>%s &middot; %s &middot; %d devices</p><table><tr>",
			html.EscapeString(rangeDesc), time.Now().Format("2006-01-02 15:04"), len(devs))
		for _, c := range cols {
			sb.WriteString("<th>" + html.EscapeString(T(c.Title)) + "</th>")
		}
		sb.WriteString("</tr>")
		for _, d := range devs {
			cls := ""
			if !d.Alive {
				cls = " class=\"off\""
			}
			sb.WriteString("<tr" + cls + ">")
			for _, c := range cols {
				sb.WriteString("<td>" + html.EscapeString(c.Get(d, cfg)) + "</td>")
			}
			sb.WriteString("</tr>")
			for _, s := range d.Shares {
				fmt.Fprintf(&sb, "<tr class=\"share\"><td class=\"sh\" colspan=\"%d\">\\\\%s\\%s &mdash; %s %s</td></tr>", len(cols),
					d.IP, html.EscapeString(s.Name), s.Kind(), html.EscapeString(s.Remark))
			}
		}
		sb.WriteString("</table></body></html>")
		_, err := io.WriteString(out, sb.String())
		return err
	default: // .txt
		widths := make([]int, len(cols))
		rows := [][]string{}
		hdr := []string{}
		for i, c := range cols {
			hdr = append(hdr, T(c.Title))
			widths[i] = len([]rune(T(c.Title)))
		}
		for _, d := range devs {
			r := []string{}
			for i, c := range cols {
				v := c.Get(d, cfg)
				r = append(r, v)
				if l := len([]rune(v)); l > widths[i] {
					widths[i] = l
				}
			}
			rows = append(rows, r)
		}
		var sb strings.Builder
		line := func(r []string) {
			for i, v := range r {
				sb.WriteString(v)
				if i < len(r)-1 {
					sb.WriteString(strings.Repeat(" ", widths[i]-len([]rune(v))+2))
				}
			}
			sb.WriteString("\r\n")
		}
		line(hdr)
		total := 0
		for _, w := range widths {
			total += w + 2
		}
		sb.WriteString(strings.Repeat("-", total) + "\r\n")
		for i, r := range rows {
			line(r)
			for _, s := range devs[i].Shares {
				sb.WriteString(fmt.Sprintf("    \\\\%s\\%s  (%s) %s\r\n", devs[i].IP, s.Name, s.Kind(), s.Remark))
			}
		}
		_, err := io.WriteString(out, sb.String())
		return err
	}
}

func loadResults(path string) ([]*Device, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	return loadResultsFrom(f, strings.ToLower(filepath.Ext(path)))
}

func loadResultsFrom(in io.Reader, ext string) ([]*Device, string, error) {
	b, err := io.ReadAll(in)
	if err != nil {
		return nil, "", err
	}
	var r xmlResults
	if ext == ".json" || (len(b) > 0 && b[0] == '{') {
		err = json.Unmarshal(b, &r)
	} else {
		err = xml.Unmarshal(b, &r)
	}
	if err != nil {
		return nil, "", fmt.Errorf("unsupported or damaged file: %v", err)
	}
	var out []*Device
	for _, x := range r.Devices {
		if d := fromXML(x); d.IP != nil {
			out = append(out, d)
		}
	}
	return out, r.Range, nil
}
