package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/win"

	"netzscan/third_party/walk"
	. "netzscan/third_party/walk/declarative"
)

// ---------- generic dialogs ----------

func okCancel(dlg **walk.Dialog, okBtn, cancelBtn **walk.PushButton, onOK func() bool) Composite {
	return Composite{
		Layout: HBox{MarginsZero: true},
		Children: []Widget{
			HSpacer{},
			PushButton{AssignTo: okBtn, Text: T("OK"), MinSize: Size{Width: 88}, OnClicked: func() {
				if onOK == nil || onOK() {
					(*dlg).Accept()
				}
			}},
			PushButton{AssignTo: cancelBtn, Text: T("Cancel"), MinSize: Size{Width: 88}, OnClicked: func() { (*dlg).Cancel() }},
		},
	}
}

func inputBox(owner walk.Form, title, prompt, initial string) (string, bool) {
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	var ed *walk.LineEdit
	res := ""
	r, _ := Dialog{
		AssignTo: &dlg, Title: title, DefaultButton: &ok, CancelButton: &cancel,
		MinSize: Size{Width: 380}, Layout: VBox{},
		Children: []Widget{
			Label{Text: prompt},
			LineEdit{AssignTo: &ed, Text: initial},
			okCancel(&dlg, &ok, &cancel, func() bool { res = ed.Text(); return true }),
		},
	}.Run(owner)
	return res, r == walk.DlgCmdOK
}

func comboBox(owner walk.Form, title, prompt string, items []string) (string, bool) {
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	var cb *walk.ComboBox
	res := ""
	r, _ := Dialog{
		AssignTo: &dlg, Title: title, DefaultButton: &ok, CancelButton: &cancel,
		MinSize: Size{Width: 340}, Layout: VBox{},
		Children: []Widget{
			Label{Text: prompt},
			ComboBox{AssignTo: &cb, Model: items, CurrentIndex: 0},
			okCancel(&dlg, &ok, &cancel, func() bool { res = cb.Text(); return true }),
		},
	}.Run(owner)
	return res, r == walk.DlgCmdOK
}

func listChoice(owner walk.Form, title, prompt string, items []string) (int, bool) {
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	var lb *walk.ListBox
	res := 0
	r, _ := Dialog{
		AssignTo: &dlg, Title: title, DefaultButton: &ok, CancelButton: &cancel,
		MinSize: Size{Width: 520, Height: 260}, Layout: VBox{},
		Children: []Widget{
			Label{Text: prompt},
			ListBox{AssignTo: &lb, Model: items, CurrentIndex: 0, OnItemActivated: func() { res = lb.CurrentIndex(); dlg.Accept() }},
			okCancel(&dlg, &ok, &cancel, func() bool { res = lb.CurrentIndex(); return res >= 0 }),
		},
	}.Run(owner)
	return res, r == walk.DlgCmdOK
}

func textListDialog(owner walk.Form, title, prompt string, lines []string) ([]string, bool) {
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	var te *walk.TextEdit
	var res []string
	r, _ := Dialog{
		AssignTo: &dlg, Title: title, DefaultButton: nil, CancelButton: &cancel,
		MinSize: Size{Width: 460, Height: 360}, Layout: VBox{},
		Children: []Widget{
			Label{Text: prompt},
			TextEdit{AssignTo: &te, Text: strings.Join(lines, "\r\n"), VScroll: true, Font: Font{Family: "Consolas", PointSize: 10}},
			okCancel(&dlg, &ok, &cancel, func() bool {
				res = nil
				for _, l := range strings.Split(strings.ReplaceAll(te.Text(), "\r", ""), "\n") {
					if l = strings.TrimSpace(l); l != "" {
						res = append(res, l)
					}
				}
				return true
			}),
		},
	}.Run(owner)
	return res, r == walk.DlgCmdOK
}

// ---------- simple table model for result windows ----------

type itemModel struct {
	walk.TableModelBase
	walk.SorterBase
	items []DiscoveryItem
}

func (m *itemModel) RowCount() int { return len(m.items) }
func (m *itemModel) Value(r, c int) interface{} {
	if r < len(m.items) && c < len(m.items[r].Cols) {
		return m.items[r].Cols[c]
	}
	return ""
}
func (m *itemModel) Sort(col int, order walk.SortOrder) error {
	sort.SliceStable(m.items, func(i, j int) bool {
		a, b := m.items[i].Cols[col], m.items[j].Cols[col]
		ia, ib := net.ParseIP(a), net.ParseIP(b)
		var less bool
		if ia != nil && ib != nil && ia.To4() != nil && ib.To4() != nil {
			less = ipToU32(ia) < ipToU32(ib)
		} else {
			less = strings.ToLower(a) < strings.ToLower(b)
		}
		if order == walk.SortDescending {
			return !less
		}
		return less
	})
	m.PublishRowsReset()
	return m.SorterBase.Sort(col, order)
}

// resultTable shows a list of items; if refresh != nil a "Refresh" button reruns it.
func resultTable(owner walk.Form, title string, cols []string, items []DiscoveryItem, refresh func() ([]DiscoveryItem, error)) {
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	var tv *walk.TableView
	var lbl *walk.Label
	var btnRefresh *walk.PushButton
	m := &itemModel{items: items}
	var tvc []TableViewColumn
	for i, c := range cols {
		w := 130
		if i == len(cols)-1 {
			w = 240
		}
		tvc = append(tvc, TableViewColumn{Title: c, Width: w})
	}
	copySel := func(all bool) {
		var lines []string
		lines = append(lines, strings.Join(cols, "\t"))
		idx := tv.SelectedIndexes()
		if all {
			idx = nil
			for i := range m.items {
				idx = append(idx, i)
			}
		}
		for _, i := range idx {
			lines = append(lines, strings.Join(m.items[i].Cols, "\t"))
		}
		walk.Clipboard().SetText(strings.Join(lines, "\r\n"))
	}
	doRefresh := func() {
		btnRefresh.SetEnabled(false)
		lbl.SetText(T("Searching..."))
		go func() {
			it, err := refresh()
			dlg.Synchronize(func() {
				btnRefresh.SetEnabled(true)
				if err != nil {
					lbl.SetText(T("Error: ") + err.Error())
					return
				}
				m.items = it
				m.PublishRowsReset()
				lbl.SetText(fmt.Sprintf(T("%d item(s) found"), len(it)))
			})
		}()
	}
	Dialog{
		AssignTo: &dlg, CancelButton: &closeBtn, Title: title, MinSize: Size{Width: 860, Height: 460}, Layout: VBox{},
		Children: []Widget{
			TableView{AssignTo: &tv, Model: m, Columns: tvc, MultiSelection: true, AlternatingRowBG: true,
				ContextMenuItems: []MenuItem{
					Action{Text: T("Copy Selected"), OnTriggered: func() { copySel(false) }},
					Action{Text: T("Copy All"), OnTriggered: func() { copySel(true) }},
					Action{Text: T("Open in Browser"), OnTriggered: func() {
						if i := tv.CurrentIndex(); i >= 0 {
							shellOpen("http://"+m.items[i].Cols[0]+"/", "")
						}
					}},
				}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{AssignTo: &lbl, Text: fmt.Sprintf(T("%d item(s)"), len(items))},
				HSpacer{},
				PushButton{AssignTo: &btnRefresh, Text: T("Refresh"), Visible: refresh != nil, OnClicked: doRefresh},
				PushButton{Text: T("Copy All"), OnClicked: func() { copySel(true) }},
				PushButton{AssignTo: &closeBtn, Text: T("Close"), OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(owner)
	if refresh != nil && items == nil {
		doRefresh()
	}
	dlg.Run()
}

// ---------- App dialogs ----------

func (a *App) extraRangesDlg() {
	lines, ok := textListDialog(a.mw, T("Additional Ranges"),
		T("One entry per line. Supported formats:\r\n  192.168.2.1-192.168.2.254   ·   10.0.0.0/24   ·   192.168.5.10   ·   host name\r\nThese are scanned together with the From/To range."), a.cfg.ExtraRanges)
	if !ok {
		return
	}
	for _, l := range lines {
		if _, err := parseRangeSpec(l); err != nil {
			walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconWarning)
			return
		}
	}
	a.cfg.ExtraRanges = lines
	a.updateExtraLabel()
}

func (a *App) ignoredDlg() {
	lines, ok := textListDialog(a.mw, T("Ignored IP Addresses"), T("These addresses (single IPs, ranges or CIDR) are skipped during scans:"), a.cfg.Ignored)
	if ok {
		a.cfg.Ignored = lines
	}
}

func (a *App) visibleColumnsDlg() {
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	checks := make([]*walk.CheckBox, len(allColumns))
	var kids []Widget
	for i, c := range allColumns {
		i, c := i, c
		title := T(c.Title)
		if !c.Default && c.Auto != nil {
			title += T("  (when enabled in options)")
		}
		kids = append(kids, CheckBox{AssignTo: &checks[i], Text: title, Checked: !a.cfg.HiddenColumns[c.ID], Enabled: c.ID != "ip"})
	}
	kids = append(kids, VSpacer{}, okCancel(&dlg, &ok, &cancel, func() bool {
		for i, c := range allColumns {
			if c.ID == "ip" {
				continue
			}
			if checks[i].Checked() {
				delete(a.cfg.HiddenColumns, c.ID)
			} else {
				a.cfg.HiddenColumns[c.ID] = true
			}
		}
		return true
	}))
	if r, _ := (Dialog{AssignTo: &dlg, Title: T("Visible Columns"), DefaultButton: &ok, CancelButton: &cancel, MinSize: Size{Width: 360}, Layout: VBox{}, Children: kids}).Run(a.mw); r == walk.DlgCmdOK {
		a.applyColumnVisibility()
	}
}

func (a *App) legendDlg() {
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	var iv []*walk.ImageView
	entries := [][2]string{
		{"dot_on", T("Device is online (responded to ping, ARP or TCP)")},
		{"dot_new", T("New device – seen for the first time")},
		{"dot_off", T("Device did not respond (shown when 'Display inactive devices' is on)")},
		{"dot_on_x", T("Device has shared folders / child items – click to expand")},
		{"folder", T("Shared folder")},
		{"printer", T("Shared printer")},
		{"port", T("Open TCP port (when 'Show ports as tree nodes' is on)")},
	}
	iv = make([]*walk.ImageView, len(entries))
	var kids []Widget
	for i, e := range entries {
		kids = append(kids, ImageView{AssignTo: &iv[i], MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeIdeal}, Label{Text: e[1]})
	}
	Dialog{AssignTo: &dlg, CancelButton: &closeBtn, Title: T("Legend"), Layout: Grid{Columns: 2, Spacing: 8}, MinSize: Size{Width: 440},
		Children: append(kids, HSpacer{}, PushButton{AssignTo: &closeBtn, Text: T("Close"), OnClicked: func() { dlg.Cancel() }})}.Create(a.mw)
	for i, e := range entries {
		iv[i].SetImage(smallIcon(e[0], a.dpi))
	}
	dlg.Run()
}

func (a *App) aboutDlg() {
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	var img *walk.ImageView
	Dialog{AssignTo: &dlg, CancelButton: &closeBtn, Title: T("About ") + appName, Layout: VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 12}}, MinSize: Size{Width: 420},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true, Spacing: 14}, Children: []Widget{
				ImageView{AssignTo: &img, Mode: ImageViewModeIdeal, MinSize: Size{Width: 64, Height: 64}, MaxSize: Size{Width: 64, Height: 64}},
				Composite{Layout: VBox{MarginsZero: true}, Children: []Widget{
					Label{Text: appName + T(" Network Scanner"), Font: Font{PointSize: 13, Bold: true}},
					Label{Text: T("Version ") + appVersion + T(" (64-bit)")},
					Label{Text: T("Fast multi-threaded IPv4 scanner: ping, ARP, DNS/NetBIOS,\nMAC vendors, TCP ports, SMB shares, Wake-on-LAN,\nUPnP / mDNS / DHCP discovery.")},
				}},
			}},
			Label{Text: T("Icons: Icons8 Flat Color Icons and Microsoft Fluent UI (MIT).\nMAC vendor data: IEEE OUI registry."), TextColor: walk.RGB(0x66, 0x66, 0x66)},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &closeBtn, Text: T("OK"), OnClicked: func() { dlg.Accept() }}}},
		}}.Create(a.mw)
	img.SetImage(bitmapPx("app", 64*a.dpi/96, a.dpi))
	dlg.Run()
}

func (a *App) helpDlg() {
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	text := helpText()
	Dialog{AssignTo: &dlg, CancelButton: &closeBtn, Title: appName + T(" Help"), Layout: VBox{}, MinSize: Size{Width: 720, Height: 520},
		Children: []Widget{
			TextEdit{Text: strings.ReplaceAll(text, "\n", "\r\n"), ReadOnly: true, VScroll: true, Font: Font{Family: "Consolas", PointSize: 10}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{HSpacer{}, PushButton{AssignTo: &closeBtn, Text: T("Close"), OnClicked: func() { dlg.Cancel() }}}},
		}}.Run(a.mw)
}

func (a *App) propertiesDlg() {
	rw, ok := a.currentRow()
	if !ok {
		return
	}
	d := rw.d
	var b strings.Builder
	add := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%-18s %s\r\n", k+":", v)
		}
	}
	if rw.kind == rowShare {
		s := d.Shares[rw.idx]
		add(T("Share"), `\\`+d.IP.String()+`\`+s.Name)
		add(T("Type"), s.Kind())
		add(T("Remark"), s.Remark)
		b.WriteString("\r\n")
	}
	add(T("IP address"), d.IP.String())
	status := T("Online")
	if !d.Alive {
		status = T("No response")
	}
	add(T("Status"), status)
	add(T("Detected by"), d.Method)
	if d.RTT >= 0 && d.Alive {
		add(T("Response time"), fmt.Sprintf("%d ms", d.RTT))
	}
	if d.TTL > 0 {
		add("TTL", strconv.Itoa(d.TTL))
	}
	add(T("Host name"), d.HostName)
	add(T("NetBIOS name"), d.NetBIOS)
	add(T("Workgroup"), d.Workgroup)
	add(T("User"), d.User)
	add(T("MAC address"), formatMAC(d.MAC, a.cfg.MACFormat))
	add(T("MAC vendor"), d.Vendor)
	add(T("Device type"), d.DeviceType)
	add(T("Comment"), friendlyName(d, a.cfg))
	if len(d.Ports) > 0 {
		var ps []string
		for _, p := range d.Ports {
			if n := serviceName(p); n != "" {
				ps = append(ps, fmt.Sprintf("%d (%s)", p, n))
			} else {
				ps = append(ps, strconv.Itoa(p))
			}
		}
		add(T("Open TCP ports"), strings.Join(ps, ", "))
	}
	add(T("HTTP server"), d.HTTPServer)
	add(T("Web page title"), d.HTTPTitle)
	for i, s := range d.Shares {
		k := ""
		if i == 0 {
			k = T("Shares")
		}
		fmt.Fprintf(&b, "%-18s %s  (%s) %s\r\n", k+func() string {
			if k != "" {
				return ":"
			}
			return ""
		}(), s.Name, s.Kind(), s.Remark)
	}
	if !d.LastScan.IsZero() {
		add(T("Last scan"), d.LastScan.Format("2006-01-02 15:04:05"))
	}
	if d.MAC != nil {
		if fs, ok := a.cfg.KnownDevices[formatMAC(d.MAC, 0)]; ok {
			add(T("First seen"), fs)
		}
	}
	var dlg *walk.Dialog
	var closeBtn *walk.PushButton
	var te *walk.TextEdit
	Dialog{AssignTo: &dlg, CancelButton: &closeBtn, Title: T("Properties – ") + d.IP.String(), Layout: VBox{}, MinSize: Size{Width: 560, Height: 420},
		Children: []Widget{
			TextEdit{AssignTo: &te, Text: b.String(), ReadOnly: true, VScroll: true, Font: Font{Family: "Consolas", PointSize: 10}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: T("Copy"), OnClicked: func() { walk.Clipboard().SetText(b.String()) }},
				HSpacer{}, PushButton{AssignTo: &closeBtn, Text: T("Close"), OnClicked: func() { dlg.Cancel() }}}},
		}}.Create(a.mw)
	te.SetTextSelection(0, 0)
	closeBtn.SetFocus()
	dlg.Run()
}

func friendlyKey(d *Device) string {
	if d.MAC != nil {
		return formatMAC(d.MAC, 0)
	}
	return d.IP.String()
}

func (a *App) assignFriendly() {
	ds := a.selectedDevices()
	if len(ds) == 0 {
		return
	}
	cur := friendlyName(ds[0], a.cfg)
	name, ok := inputBox(a.mw, T("Assign Friendly Name"), T("Friendly name / comment for ")+ds[0].IP.String()+T(" (bound to its MAC address when known):"), cur)
	if !ok {
		return
	}
	for _, d := range ds {
		if strings.TrimSpace(name) == "" {
			delete(a.cfg.FriendlyNames, friendlyKey(d))
		} else {
			a.cfg.FriendlyNames[friendlyKey(d)] = strings.TrimSpace(name)
		}
	}
	a.applyColumnVisibility()
	a.refresh()
	a.cfg.save("")
}

func (a *App) friendlyNamesDlg() {
	var keys []string
	for k := range a.cfg.FriendlyNames {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, k+" = "+a.cfg.FriendlyNames[k])
	}
	res, ok := textListDialog(a.mw, T("Friendly Names / Comments"), T("One entry per line:  MAC-or-IP = name"), lines)
	if !ok {
		return
	}
	m := map[string]string{}
	for _, l := range res {
		if i := strings.Index(l, "="); i > 0 {
			k, v := strings.TrimSpace(l[:i]), strings.TrimSpace(l[i+1:])
			if mac, err := parseMAC(k); err == nil {
				k = formatMAC(mac, 0)
			}
			if v != "" {
				m[k] = v
			}
		}
	}
	a.cfg.FriendlyNames = m
	a.applyColumnVisibility()
	a.refresh()
	a.cfg.save("")
}

// ---------- bookmarks ----------

func (a *App) addBookmark() {
	from, to := a.ipFrom.IPString(), a.ipTo.IPString()
	name, ok := inputBox(a.mw, T("Add Bookmark"), T("Bookmark name:"), from+" - "+to)
	if !ok {
		return
	}
	a.cfg.Bookmarks = append(a.cfg.Bookmarks, Bookmark{Name: name, From: from, To: to, Extra: append([]string{}, a.cfg.ExtraRanges...)})
	a.refreshBookmarksMenu()
	a.cfg.save("")
}

func (a *App) manageBookmarks() {
	var lines []string
	for _, b := range a.cfg.Bookmarks {
		l := b.Name + " | " + b.From + " - " + b.To
		if len(b.Extra) > 0 {
			l += " | " + strings.Join(b.Extra, ", ")
		}
		lines = append(lines, l)
	}
	res, ok := textListDialog(a.mw, T("Manage Bookmarks"), T("One bookmark per line:  name | from - to | optional extra ranges (comma separated)"), lines)
	if !ok {
		return
	}
	var bms []Bookmark
	for _, l := range res {
		parts := strings.Split(l, "|")
		if len(parts) < 2 {
			continue
		}
		rng := strings.Split(parts[1], "-")
		b := Bookmark{Name: strings.TrimSpace(parts[0]), From: strings.TrimSpace(rng[0])}
		b.To = b.From
		if len(rng) > 1 {
			b.To = strings.TrimSpace(rng[1])
		}
		if len(parts) > 2 {
			for _, e := range strings.Split(parts[2], ",") {
				if e = strings.TrimSpace(e); e != "" {
					b.Extra = append(b.Extra, e)
				}
			}
		}
		if net.ParseIP(b.From) != nil {
			bms = append(bms, b)
		}
	}
	a.cfg.Bookmarks = bms
	a.refreshBookmarksMenu()
	a.cfg.save("")
}

// ---------- Wake-on-LAN ----------

func (a *App) wolSelected() {
	n := 0
	var errs []string
	for _, d := range a.selectedDevices() {
		if d.MAC == nil {
			errs = append(errs, d.IP.String()+T(": MAC address unknown"))
			continue
		}
		if err := sendWOL(d.MAC); err != nil {
			errs = append(errs, d.IP.String()+": "+err.Error())
		} else {
			n++
		}
	}
	msg := fmt.Sprintf(T("Wake-On-LAN signal sent to %d device(s)."), n)
	if len(errs) > 0 {
		msg += "\n\n" + strings.Join(errs, "\n")
	}
	walk.MsgBox(a.mw, T("Wake-On-LAN"), msg, walk.MsgBoxIconInformation)
}

func (a *App) saveMACToWOL() {
	n := 0
	for _, d := range a.selectedDevices() {
		if d.MAC == nil {
			continue
		}
		mac := formatMAC(d.MAC, 0)
		exists := false
		for _, w := range a.cfg.WOL {
			if strings.EqualFold(w.MAC, mac) {
				exists = true
			}
		}
		if !exists {
			name := friendlyName(d, a.cfg)
			if name == "" {
				name = d.HostName
			}
			a.cfg.WOL = append(a.cfg.WOL, WOLEntry{Name: name, MAC: mac, IP: d.IP.String()})
			n++
		}
	}
	a.cfg.save("")
	walk.MsgBox(a.mw, T("Wake-On-LAN Manager"), fmt.Sprintf(T("%d MAC address(es) added to the Wake-On-LAN Manager."), n), walk.MsgBoxIconInformation)
}

type wolModel struct {
	walk.TableModelBase
	items *[]WOLEntry
}

func (m *wolModel) RowCount() int { return len(*m.items) }
func (m *wolModel) Value(r, c int) interface{} {
	e := (*m.items)[r]
	return []string{e.Name, e.MAC, e.IP, e.Broadcast}[c]
}

func (a *App) wolManager() {
	var dlg *walk.Dialog
	var tv *walk.TableView
	var edName, edMAC, edIP, edBC *walk.LineEdit
	m := &wolModel{items: &a.cfg.WOL}
	sel := func() []int { return tv.SelectedIndexes() }
	Dialog{AssignTo: &dlg, Title: T("Wake-On-LAN Manager"), Layout: VBox{}, MinSize: Size{Width: 700, Height: 440},
		Children: []Widget{
			TableView{AssignTo: &tv, Model: m, MultiSelection: true, Columns: []TableViewColumn{
				{Title: T("Name"), Width: 200}, {Title: T("MAC address"), Width: 150}, {Title: T("IP address"), Width: 120}, {Title: T("Broadcast / target"), Width: 140}},
				OnCurrentIndexChanged: func() {
					if i := tv.CurrentIndex(); i >= 0 {
						e := a.cfg.WOL[i]
						edName.SetText(e.Name)
						edMAC.SetText(e.MAC)
						edIP.SetText(e.IP)
						edBC.SetText(e.Broadcast)
					}
				}},
			Composite{Layout: Grid{Columns: 4, MarginsZero: true}, Children: []Widget{
				Label{Text: T("Name")}, LineEdit{AssignTo: &edName}, Label{Text: T("MAC address")}, LineEdit{AssignTo: &edMAC},
				Label{Text: T("IP address")}, LineEdit{AssignTo: &edIP}, Label{Text: T("Broadcast (optional)")}, LineEdit{AssignTo: &edBC},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: T("Add"), OnClicked: func() {
					mac, err := parseMAC(edMAC.Text())
					if err != nil {
						walk.MsgBox(dlg, T("Wake-On-LAN"), err.Error(), walk.MsgBoxIconWarning)
						return
					}
					a.cfg.WOL = append(a.cfg.WOL, WOLEntry{Name: edName.Text(), MAC: formatMAC(mac, 0), IP: edIP.Text(), Broadcast: edBC.Text()})
					m.PublishRowsReset()
				}},
				PushButton{Text: T("Update"), OnClicked: func() {
					i := tv.CurrentIndex()
					mac, err := parseMAC(edMAC.Text())
					if i < 0 || err != nil {
						return
					}
					a.cfg.WOL[i] = WOLEntry{Name: edName.Text(), MAC: formatMAC(mac, 0), IP: edIP.Text(), Broadcast: edBC.Text()}
					m.PublishRowsReset()
				}},
				PushButton{Text: T("Remove"), OnClicked: func() {
					s := sel()
					sort.Sort(sort.Reverse(sort.IntSlice(s)))
					for _, i := range s {
						a.cfg.WOL = append(a.cfg.WOL[:i], a.cfg.WOL[i+1:]...)
					}
					m.PublishRowsReset()
				}},
				HSpacer{},
				PushButton{Text: T("Wake Up Selected"), OnClicked: func() {
					n := 0
					for _, i := range sel() {
						e := a.cfg.WOL[i]
						if mac, err := parseMAC(e.MAC); err == nil {
							if sendWOL(mac, e.Broadcast, e.IP) == nil {
								n++
							}
						}
					}
					walk.MsgBox(dlg, T("Wake-On-LAN"), fmt.Sprintf(T("Signal sent to %d device(s)."), n), walk.MsgBoxIconInformation)
				}},
				PushButton{Text: T("Close"), OnClicked: func() { dlg.Accept() }},
			}},
		}}.Run(a.mw)
	a.cfg.save("")
}

// ---------- remote actions ----------

func hiddenCmd(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return c
}

func (a *App) shutdownDlg() {
	ds := a.selectedDevices()
	if len(ds) == 0 {
		return
	}
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton
	var cbMode *walk.ComboBox
	var neTimeout *walk.NumberEdit
	var cbForce *walk.CheckBox
	var edMsg *walk.LineEdit
	var targets []string
	for _, d := range ds {
		targets = append(targets, d.IP.String())
	}
	r, _ := Dialog{AssignTo: &dlg, Title: T("Remote Shutdown"), DefaultButton: &ok, CancelButton: &cancel, Layout: Grid{Columns: 2}, MinSize: Size{Width: 440},
		Children: []Widget{
			Label{Text: T("Devices:")}, Label{Text: strings.Join(targets, ", ")},
			Label{Text: T("Action:")}, ComboBox{AssignTo: &cbMode, Model: []string{T("Shut down"), T("Restart"), T("Cancel pending shutdown")}, CurrentIndex: 0},
			Label{Text: T("Timeout (sec):")}, NumberEdit{AssignTo: &neTimeout, Value: 30.0, MinValue: 0, MaxValue: 315360000},
			Label{Text: T("Message:")}, LineEdit{AssignTo: &edMsg},
			HSpacer{}, CheckBox{AssignTo: &cbForce, Text: T("Force running applications to close")},
			Label{Text: T("Uses Windows shutdown.exe; requires administrative rights on the target."), ColumnSpan: 2, TextColor: walk.RGB(0x66, 0x66, 0x66)},
			Composite{ColumnSpan: 2, Layout: HBox{MarginsZero: true}, Children: []Widget{HSpacer{},
				PushButton{AssignTo: &ok, Text: T("OK"), OnClicked: func() { dlg.Accept() }},
				PushButton{AssignTo: &cancel, Text: T("Cancel"), OnClicked: func() { dlg.Cancel() }}}},
		}}.Run(a.mw)
	if r != walk.DlgCmdOK {
		return
	}
	mode := []string{"/s", "/r", "/a"}[cbMode.CurrentIndex()]
	var errs []string
	for _, t := range targets {
		args := []string{mode, "/m", `\\` + t}
		if mode != "/a" {
			args = append(args, "/t", strconv.Itoa(int(neTimeout.Value())))
			if cbForce.Checked() {
				args = append(args, "/f")
			}
			if edMsg.Text() != "" {
				args = append(args, "/c", edMsg.Text())
			}
		}
		if out, err := hiddenCmd("shutdown", args...).CombinedOutput(); err != nil {
			errs = append(errs, t+": "+strings.TrimSpace(string(out)))
		}
	}
	if len(errs) > 0 {
		walk.MsgBox(a.mw, T("Remote Shutdown"), strings.Join(errs, "\n"), walk.MsgBoxIconWarning)
	} else {
		walk.MsgBox(a.mw, T("Remote Shutdown"), T("Command sent successfully."), walk.MsgBoxIconInformation)
	}
}

func (a *App) sendMessageDlg() {
	ds := a.selectedDevices()
	if len(ds) == 0 {
		return
	}
	text, ok := inputBox(a.mw, T("Send Message"), T("Message to display on the selected Windows computer(s) (uses msg.exe):"), "")
	if !ok || strings.TrimSpace(text) == "" {
		return
	}
	var errs []string
	for _, d := range ds {
		if out, err := hiddenCmd("msg", "*", "/server:"+d.IP.String(), text).CombinedOutput(); err != nil {
			errs = append(errs, d.IP.String()+": "+strings.TrimSpace(string(out))+" "+err.Error())
		}
	}
	if len(errs) > 0 {
		walk.MsgBox(a.mw, T("Send Message"), strings.Join(errs, "\n"), walk.MsgBoxIconWarning)
	}
}

// ---------- discovery ----------

func (a *App) upnpDiscovery() {
	resultTable(a.mw, T("UPnP Device Discovery"), []string{T("IP address"), T("Name"), T("Manufacturer"), T("Model"), T("Device type"), T("Server"), T("Location")}, nil,
		func() ([]DiscoveryItem, error) { return discoverUPnP(4 * time.Second) })
}

func (a *App) mdnsDiscovery() {
	resultTable(a.mw, T("mDNS Device Discovery"), []string{T("IP address"), T("Instance"), T("Service"), T("Host"), T("Port"), "TXT"}, nil,
		func() ([]DiscoveryItem, error) { return discoverMDNS(4 * time.Second) })
}

func (a *App) dhcpDiscovery() {
	resultTable(a.mw, T("DHCP Server Discovery"), []string{T("DHCP server"), T("Offered IP"), T("Subnet mask"), T("Router"), T("DNS servers"), T("Domain"), T("Lease time")}, nil,
		func() ([]DiscoveryItem, error) { return discoverDHCP(5 * time.Second) })
}

// ---------- reports ----------

func (a *App) compareDlg() {
	if len(a.model.devices) == 0 {
		walk.MsgBox(a.mw, appName, T("Perform a scan or load results first. The current results are compared with a saved scan."), walk.MsgBoxIconInformation)
		return
	}
	dlg := &walk.FileDialog{Title: T("Select a saved scan to compare with"), Filter: T("Scan results (*.xml;*.json)|*.xml;*.json")}
	if ok, _ := dlg.ShowOpen(a.mw); !ok {
		return
	}
	old, _, err := loadResults(dlg.FilePath)
	if err != nil {
		walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
		return
	}
	om := map[string]*Device{}
	for _, d := range old {
		om[d.IP.String()] = d
	}
	var items []DiscoveryItem
	seen := map[string]bool{}
	for _, d := range a.model.devices {
		k := d.IP.String()
		seen[k] = true
		o, ok := om[k]
		switch {
		case !ok && d.Alive:
			items = append(items, DiscoveryItem{Cols: []string{k, T("New"), d.HostName, formatMAC(d.MAC, a.cfg.MACFormat), ""}})
		case ok && o.Alive && !d.Alive:
			items = append(items, DiscoveryItem{Cols: []string{k, T("Went offline"), o.HostName, formatMAC(o.MAC, a.cfg.MACFormat), ""}})
		case ok && !o.Alive && d.Alive:
			items = append(items, DiscoveryItem{Cols: []string{k, T("Came online"), d.HostName, formatMAC(d.MAC, a.cfg.MACFormat), ""}})
		case ok:
			var ch []string
			if formatMAC(o.MAC, 0) != formatMAC(d.MAC, 0) && o.MAC != nil && d.MAC != nil {
				ch = append(ch, "MAC "+formatMAC(o.MAC, 0)+" → "+formatMAC(d.MAC, 0))
			}
			if o.HostName != d.HostName && o.HostName != "" && d.HostName != "" {
				ch = append(ch, T("Name ")+o.HostName+" → "+d.HostName)
			}
			if portsString(o, a.cfg) != portsString(d, a.cfg) {
				ch = append(ch, T("Ports [")+portsString(o, a.cfg)+"] → ["+portsString(d, a.cfg)+"]")
			}
			if len(ch) > 0 {
				items = append(items, DiscoveryItem{Cols: []string{k, T("Changed"), d.HostName, formatMAC(d.MAC, a.cfg.MACFormat), strings.Join(ch, "; ")}})
			}
		}
	}
	for k, o := range om {
		if !seen[k] && o.Alive {
			items = append(items, DiscoveryItem{Cols: []string{k, T("Missing"), o.HostName, formatMAC(o.MAC, a.cfg.MACFormat), T("not in current results")}})
		}
	}
	sortItemsByIP(items)
	if len(items) == 0 {
		walk.MsgBox(a.mw, T("Compare Scans"), T("No differences found."), walk.MsgBoxIconInformation)
		return
	}
	resultTable(a.mw, T("Compare Scans – current vs. ")+dlg.FilePath, []string{T("IP address"), T("Change"), T("Host name"), T("MAC address"), T("Details")}, items, nil)
}

func (a *App) summaryDlg() {
	byType := map[string]int{}
	byVendor := map[string]int{}
	ports := map[int]int{}
	online := 0
	shares := 0
	for _, d := range a.model.devices {
		if !d.Alive {
			continue
		}
		online++
		t := d.DeviceType
		if t == "" {
			t = T("(unknown)")
		}
		byType[t]++
		v := d.Vendor
		if v == "" {
			v = T("(unknown)")
		}
		byVendor[v]++
		for _, p := range d.Ports {
			ports[p]++
		}
		shares += len(d.Shares)
	}
	var items []DiscoveryItem
	items = append(items, DiscoveryItem{Cols: []string{T("Total"), T("Online devices"), strconv.Itoa(online)}})
	items = append(items, DiscoveryItem{Cols: []string{T("Total"), T("Shared resources"), strconv.Itoa(shares)}})
	addMap := func(cat string, m map[string]int) {
		type kv struct {
			k string
			v int
		}
		var l []kv
		for k, v := range m {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v || (l[i].v == l[j].v && l[i].k < l[j].k) })
		for _, x := range l {
			items = append(items, DiscoveryItem{Cols: []string{cat, x.k, strconv.Itoa(x.v)}})
		}
	}
	addMap(T("Device type"), byType)
	addMap(T("Vendor"), byVendor)
	pm := map[string]int{}
	for p, n := range ports {
		k := strconv.Itoa(p)
		if s := serviceName(p); s != "" {
			k += " (" + s + ")"
		}
		pm[k] = n
	}
	addMap(T("Open port"), pm)
	resultTable(a.mw, T("Device Summary"), []string{T("Category"), T("Value"), T("Count")}, items, nil)
}

var _ = os.Getenv
var _ = win.IDOK
