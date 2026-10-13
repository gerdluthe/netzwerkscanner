package main

import (
	"netzscan/third_party/walk"
	. "netzscan/third_party/walk/declarative"
)

func (a *App) optionsDlg() {
	c := a.cfg.clone()
	var dlg *walk.Dialog
	var ok, cancel *walk.PushButton

	var neThreads, neTimeout, neAttempts *walk.NumberEdit
	var cbMethod, cbHostMethod, cbMacFmt, cbDelim *walk.ComboBox
	cb := map[string]*walk.CheckBox{}
	chk := func(id, text string, val bool) CheckBox {
		var p *walk.CheckBox
		w := CheckBox{AssignTo: &p, Text: text, Checked: val}
		// AssignTo is resolved at creation; store pointer holder
		holders = append(holders, holder{id, &p})
		return w
	}
	holders = nil
	var edPorts *walk.LineEdit

	delims := []string{";", ",", "Tab"}
	delimIdx := 0
	for i, d := range delims {
		if d == c.CSVDelim || (d == "Tab" && c.CSVDelim == "\t") {
			delimIdx = i
		}
	}

	r, err := Dialog{
		AssignTo: &dlg, Title: T("Options"), DefaultButton: &ok, CancelButton: &cancel,
		MinSize: Size{Width: 680, Height: 520}, Layout: VBox{},
		Children: []Widget{
			TabWidget{Pages: []TabPage{
				{Title: T("General"), Layout: Grid{Columns: 3, Spacing: 10, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}}, Children: []Widget{
					Label{Text: T("Max. threads")}, Label{Text: T("Ping timeout, ms")}, Label{Text: T("Attempts")},
					NumberEdit{AssignTo: &neThreads, Value: float64(c.MaxThreads), MinValue: 1, MaxValue: 1024, SpinButtonsVisible: true},
					NumberEdit{AssignTo: &neTimeout, Value: float64(c.PingTimeout), MinValue: 50, MaxValue: 30000, SpinButtonsVisible: true},
					NumberEdit{AssignTo: &neAttempts, Value: float64(c.Attempts), MinValue: 1, MaxValue: 10, SpinButtonsVisible: true},
					Label{Text: T("Method"), ColumnSpan: 3},
					ComboBox{AssignTo: &cbMethod, Model: []string{T("ICMP only"), T("ARP only (local subnet)"), T("Both ICMP and ARP")}, CurrentIndex: c.PingMethod, ColumnSpan: 1},
					HSpacer{ColumnSpan: 2},
					VSpacer{Size: 6, ColumnSpan: 3},
					Composite{ColumnSpan: 3, Layout: Grid{Columns: 2, MarginsZero: true, Spacing: 12}, Children: []Widget{
						chk("AlwaysAnalyse", T("Always analyse device (even if ping failed)"), c.AlwaysAnalyse),
						chk("ShowPingMethod", T("Show successful ping method"), c.ShowPingMethod),
						chk("DisplayInactive", T("Display inactive devices"), c.DisplayInactive),
						chk("DisplayTTL", T("Display Time To Live (TTL)"), c.DisplayTTL),
						chk("ScanRandom", T("Scan in random order"), c.ScanRandom),
						chk("DetectDupMAC", T("Detect duplicate MAC addresses"), c.DetectDupMAC),
						chk("ShowLastScan", T("Show last-scan time"), c.ShowLastScan),
						chk("HighlightNew", T("Highlight devices seen for the first time"), c.HighlightNew),
					}},
					VSpacer{ColumnSpan: 3},
				}},
				{Title: T("Additional"), Layout: Grid{Columns: 2, Spacing: 12, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}}, Children: []Widget{
					chk("ResolveHostNames", T("Resolve host names"), c.ResolveHostNames),
					ComboBox{AssignTo: &cbHostMethod, Model: []string{T("DNS, then NetBIOS"), T("DNS only"), T("NetBIOS only")}, CurrentIndex: c.HostNameMethod},
					chk("ResolveMAC", T("Resolve MAC addresses (ARP / NetBIOS)"), c.ResolveMAC),
					chk("LookupVendor", T("Look up network card vendor"), c.LookupVendor),
					chk("DeviceTypeGuess", T("Recognise device type (offline heuristics)"), c.DeviceTypeGuess),
					chk("GrabHTTP", T("Grab HTTP server banner and page title"), c.GrabHTTP),
					VSpacer{ColumnSpan: 2},
				}},
				{Title: T("Workstation"), Layout: Grid{Columns: 2, Spacing: 12, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}}, Children: []Widget{
					chk("LookupWorkgroup", T("Look up LAN group/domain (NetBIOS)"), c.LookupWorkgroup),
					chk("LookupUser", T("Look up logged-on users (NetBIOS)"), c.LookupUser),
					VSpacer{ColumnSpan: 2},
				}},
				{Title: T("Ports"), Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10, Alignment: AlignHNearVNear}, Children: []Widget{
					GroupBox{Title: "TCP", Layout: VBox{Spacing: 8, Alignment: AlignHNearVNear}, Children: []Widget{
						chk("CheckPorts", T("Check for open TCP ports (one or more, comma- or dash-separated)"), c.CheckPorts),
						LineEdit{AssignTo: &edPorts, Text: c.Ports},
						Label{Text: T("Examples:  80,443   ·   21-23,80,443,445,3389,8080   ·   1-1024"), TextColor: walk.RGB(0x66, 0x66, 0x66)},
						chk("ServiceNames", T("Display service names rather than port numbers"), c.ServiceNames),
						chk("PortsAsNodes", T("Show open ports as tree nodes under each device"), c.PortsAsNodes),
					}},
					VSpacer{},
				}},
				{Title: T("Shares"), Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10, Alignment: AlignHNearVNear}, Children: []Widget{
					chk("EnumShares", T("Enumerate shared folders and printers (SMB)"), c.EnumShares),
					chk("ShowAdminShares", T("Show administrative shares (C$, ADMIN$ ...)"), c.ShowAdminShares),
					chk("ShowPrinterShares", T("Show shared printers"), c.ShowPrinterShares),
					Label{Text: T("Shares are listed as child nodes of each device. Uses your current Windows credentials."), TextColor: walk.RGB(0x66, 0x66, 0x66)},
					VSpacer{},
				}},
				{Title: T("View"), Layout: Grid{Columns: 2, Spacing: 12, Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}}, Children: []Widget{
					Label{Text: T("MAC address format")}, Label{Text: T("CSV file delimiter")},
					ComboBox{AssignTo: &cbMacFmt, Model: []string{"AA-BB-CC-DD-EE-FF", "AA:BB:CC:DD:EE:FF", "aabbccddeeff", "aa:bb:cc:dd:ee:ff"}, CurrentIndex: c.MACFormat},
					ComboBox{AssignTo: &cbDelim, Model: delims, CurrentIndex: delimIdx},
					chk("GridLines", T("Display grid lines"), c.GridLines),
					chk("AlternateRows", T("Alternating row colours"), c.AlternateRows),
					chk("AutoExpand", T("Auto expand tree nodes"), c.AutoExpand),
					HSpacer{},
					VSpacer{ColumnSpan: 2},
				}},
			}},
			okCancel(&dlg, &ok, &cancel, nil),
		},
	}.Run(a.mw)
	if err != nil || r != walk.DlgCmdOK {
		return
	}
	for _, h := range holders {
		if *h.p != nil {
			cb[h.id] = *h.p
		}
	}
	get := func(id string) bool { return cb[id] != nil && cb[id].Checked() }
	s := a.cfg
	settingsMu.Lock()
	s.MaxThreads = int(neThreads.Value())
	s.PingTimeout = int(neTimeout.Value())
	s.Attempts = int(neAttempts.Value())
	s.PingMethod = cbMethod.CurrentIndex()
	s.AlwaysAnalyse = get("AlwaysAnalyse")
	s.ShowPingMethod = get("ShowPingMethod")
	s.DisplayInactive = get("DisplayInactive")
	s.DisplayTTL = get("DisplayTTL")
	s.ScanRandom = get("ScanRandom")
	s.DetectDupMAC = get("DetectDupMAC")
	s.ShowLastScan = get("ShowLastScan")
	s.HighlightNew = get("HighlightNew")
	s.ResolveHostNames = get("ResolveHostNames")
	s.HostNameMethod = cbHostMethod.CurrentIndex()
	s.ResolveMAC = get("ResolveMAC")
	s.LookupVendor = get("LookupVendor")
	s.DeviceTypeGuess = get("DeviceTypeGuess")
	s.GrabHTTP = get("GrabHTTP")
	s.LookupWorkgroup = get("LookupWorkgroup")
	s.LookupUser = get("LookupUser")
	s.CheckPorts = get("CheckPorts")
	s.Ports = edPorts.Text()
	s.ServiceNames = get("ServiceNames")
	s.PortsAsNodes = get("PortsAsNodes")
	s.EnumShares = get("EnumShares")
	s.ShowAdminShares = get("ShowAdminShares")
	s.ShowPrinterShares = get("ShowPrinterShares")
	s.MACFormat = cbMacFmt.CurrentIndex()
	if d := cbDelim.Text(); d == "Tab" {
		s.CSVDelim = "\t"
	} else {
		s.CSVDelim = d
	}
	s.GridLines = get("GridLines")
	s.AlternateRows = get("AlternateRows")
	s.AutoExpand = get("AutoExpand")
	settingsMu.Unlock()
	if s.MaxThreads < 1 {
		s.MaxThreads = 1
	}
	a.ac["gridlines"].SetChecked(s.GridLines)
	a.ac["altrows"].SetChecked(s.AlternateRows)
	a.ac["autoexpand"].SetChecked(s.AutoExpand)
	a.ac["highlightnew"].SetChecked(s.HighlightNew)
	a.applyViewSettings()
	a.refresh()
	s.save("")
}

type holder struct {
	id string
	p  **walk.CheckBox
}

var holders []holder
