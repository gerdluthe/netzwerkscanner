package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"

	"netzscan/third_party/walk"
	. "netzscan/third_party/walk/declarative"
)

type App struct {
	cfg    *Settings
	mw     *walk.MainWindow
	tv     *walk.TableView
	model  *DevModel
	tb     *walk.ToolBar
	rIcons [4]*walk.ImageView

	fromHost, toHost *walk.Composite
	ipFrom, ipTo     *IPEdit
	lblExtra         *walk.Label
	btnScan          *walk.PushButton
	btnPause         *walk.PushButton
	progress         *walk.ProgressBar

	filterPanel  *walk.Composite
	cmbFilterCol *walk.ComboBox
	cmbFilterOp  *walk.ComboBox
	edFilterVal  *walk.LineEdit
	searchPanel  *walk.Composite
	edSearch     *walk.LineEdit

	sbStatus, sbThreads, sbDevices, sbTime *walk.StatusBarItem

	hostMenu, shareMenu *walk.Menu
	bookmarksMenu       *walk.Menu
	bookmarkDynamic     []*walk.Action

	scanner       *Scanner
	scanTimer     *time.Ticker
	scanDone      chan struct{}
	rescanning    int
	lastRange     string
	dpi           int
	lastClickCol  int
	firstEverScan bool

	// actions
	ac map[string]*walk.Action
}

var filterOps = []string{"contains", "does not contain", "equals", "not equal", "starts with", "ends with", "is empty", "is not empty", "greater than", "less than"}

func runGUI(cfg *Settings) {
	a := &App{cfg: cfg, model: NewDevModel(cfg), ac: map[string]*walk.Action{}}
	var cols []TableViewColumn
	for _, c := range allColumns {
		w := c.Width
		if cw, ok := cfg.ColumnWidths[c.ID]; ok && cw > 20 {
			w = cw
		}
		al := AlignDefault
		if c.Right {
			al = AlignFar
		}
		cols = append(cols, TableViewColumn{Name: c.ID, Title: T(c.Title), Width: w, Alignment: al, Hidden: !c.Visible(cfg)})
	}
	colTitles := make([]string, len(allColumns))
	for i, c := range allColumns {
		colTitles[i] = T(c.Title)
	}

	icon, _ := walk.NewIconFromResourceId(2)
	if icon == nil {
		icon, _ = walk.NewIconFromResourceId(1)
	}

	err := MainWindow{
		AssignTo: &a.mw,
		Title:    appName + T(" – Network Scanner"),
		Icon:     icon,
		Size:     Size{Width: cfg.WindowW, Height: cfg.WindowH},
		MinSize:  Size{Width: 700, Height: 300},
		Layout:   VBox{MarginsZero: true, SpacingZero: true},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &a.sbStatus, Text: T("Ready"), Width: 175},
			{AssignTo: &a.sbThreads, Text: T("Threads"), Width: 175},
			{AssignTo: &a.sbDevices, Text: T("Devices: 0 / 0"), Width: 175},
			{AssignTo: &a.sbTime, Text: "", Width: 175},
		},
		Children: []Widget{
			// IP range bar
			Composite{
				MinSize: Size{Height: 38}, MaxSize: Size{Height: 38},
				Layout: HBox{Margins: Margins{Left: 6, Top: 4, Right: 6, Bottom: 4}, Spacing: 6, Alignment: AlignHNearVCenter},
				Children: []Widget{
					Label{Text: T("IPv4 From")},
					Composite{AssignTo: &a.fromHost, Layout: HBox{MarginsZero: true}, MinSize: Size{Width: 140, Height: 23}, MaxSize: Size{Width: 140, Height: 23}},
					HSpacer{Size: 6},
					Label{Text: T("To")},
					Composite{AssignTo: &a.toHost, Layout: HBox{MarginsZero: true}, MinSize: Size{Width: 140, Height: 23}, MaxSize: Size{Width: 140, Height: 23}},
					ImageView{AssignTo: &a.rIcons[0], MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeIdeal, ToolTipText: T("Additional ranges / CIDR / host names"), OnMouseUp: func(x, y int, b walk.MouseButton) { a.extraRangesDlg() }},
					ImageView{AssignTo: &a.rIcons[1], MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeIdeal, ToolTipText: T("Auto detect local IP range"), OnMouseUp: func(x, y int, b walk.MouseButton) { a.autoDetect(false) }},
					ImageView{AssignTo: &a.rIcons[2], MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeIdeal, ToolTipText: T("Paste IP range from clipboard"), OnMouseUp: func(x, y int, b walk.MouseButton) { a.pasteIP() }},
					ImageView{AssignTo: &a.rIcons[3], MinSize: Size{Width: 20, Height: 20}, MaxSize: Size{Width: 20, Height: 20}, Mode: ImageViewModeIdeal, ToolTipText: T("Add current range to bookmarks"), OnMouseUp: func(x, y int, b walk.MouseButton) { a.addBookmark() }},
					Label{AssignTo: &a.lblExtra, Text: "", TextColor: walk.RGB(0x2e, 0x7d, 0x32)},
					HSpacer{Size: 10},
					PushButton{AssignTo: &a.btnScan, Text: T("Start Scanning"), MinSize: Size{Width: 125}, OnClicked: a.toggleScan},
					PushButton{AssignTo: &a.btnPause, Text: "", MaxSize: Size{Width: 30}, MinSize: Size{Width: 30}, Enabled: false, OnClicked: a.togglePause},
					HSpacer{},
				},
			},
			// filter panel
			Composite{
				AssignTo: &a.filterPanel, Visible: false,
				Layout: HBox{Margins: Margins{Left: 6, Top: 2, Right: 6, Bottom: 6}, Spacing: 6},
				Children: []Widget{
					Label{Text: T("Filter:")},
					ComboBox{AssignTo: &a.cmbFilterCol, Model: colTitles, CurrentIndex: 0, MinSize: Size{Width: 160}},
					ComboBox{AssignTo: &a.cmbFilterOp, Model: trList(filterOps), CurrentIndex: 0, MinSize: Size{Width: 120}},
					LineEdit{AssignTo: &a.edFilterVal, MinSize: Size{Width: 220}, OnKeyDown: func(k walk.Key) {
						if k == walk.KeyReturn {
							a.applyFilter()
						}
					}},
					PushButton{Text: T("Apply"), OnClicked: a.applyFilter},
					PushButton{Text: T("Clear"), OnClicked: a.clearFilter},
					HSpacer{},
					PushButton{Text: "×", MaxSize: Size{Width: 28}, OnClicked: func() { a.clearFilter(); a.filterPanel.SetVisible(false) }},
				},
			},
			// search panel
			Composite{
				AssignTo: &a.searchPanel, Visible: false,
				Layout: HBox{Margins: Margins{Left: 6, Top: 2, Right: 6, Bottom: 6}, Spacing: 6},
				Children: []Widget{
					PushButton{Text: "×", MaxSize: Size{Width: 28}, OnClicked: func() { a.searchPanel.SetVisible(false); a.tv.SetFocus() }},
					LineEdit{AssignTo: &a.edSearch, MinSize: Size{Width: 300}, OnKeyDown: func(k walk.Key) {
						switch k {
						case walk.KeyReturn:
							a.search(1)
						case walk.KeyEscape:
							a.searchPanel.SetVisible(false)
							a.tv.SetFocus()
						}
					}},
					PushButton{Text: T("Prev"), OnClicked: func() { a.search(-1) }},
					PushButton{Text: T("Next"), OnClicked: func() { a.search(1) }},
					HSpacer{},
				},
			},
			Composite{
				Layout: VBox{Margins: Margins{Left: 5, Right: 5}, Spacing: 2},
				Children: []Widget{
					TableView{
						AssignTo:         &a.tv,
						Model:            a.model,
						Columns:          cols,
						MultiSelection:   true,
						AlternatingRowBG: cfg.AlternateRows,
						ColumnsOrderable: true,
						StyleCell:        a.styleCell,
						OnItemActivated:  a.onActivate,
						OnKeyDown:        a.onTableKey,
						OnMouseDown:      a.onTableMouse,
						OnCurrentIndexChanged: func() {
							a.updateContextMenu()
						},
					},
					ProgressBar{AssignTo: &a.progress, MaxValue: 1000, MinSize: Size{Height: 10}, MaxSize: Size{Height: 10}},
				},
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, appName, T("Cannot create main window: ")+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.dpi = a.mw.DPI()
	a.tb = a.mw.ToolBar()

	a.ipFrom, _ = NewIPEdit(a.fromHost, a.lblExtra)
	a.ipTo, _ = NewIPEdit(a.toHost, a.lblExtra)
	a.ipFrom.SetIPString(cfg.FromIP)
	a.ipTo.SetIPString(cfg.ToIP)

	a.createActions()
	a.buildMenus()
	a.buildToolbars()
	sz := a.mw.SizePixels()
	a.mw.SetSizePixels(walk.Size{Width: sz.Width + 1, Height: sz.Height})
	a.mw.SetSizePixels(sz)
	a.applyViewSettings()
	a.updateExtraLabel()
	a.btnScan.SetImage(btnIcon("play", a.dpi))
	a.btnPause.SetImage(btnIcon("pause", a.dpi))
	a.updateStatus()

	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if a.scanner != nil {
			a.scanner.Stop()
		}
		a.saveState()
	})
	if cfg.Maximized {
		win.ShowWindow(a.mw.Handle(), win.SW_MAXIMIZE)
	}
	if cfg.FromIP == "0.0.0.0" || cfg.FromIP == "" {
		a.autoDetect(true)
	}
	a.mw.Run()
}

func (a *App) saveState() {
	ws := a.mw.SizePixels()
	var wp win.WINDOWPLACEMENT
	wp.Length = uint32(unsafe.Sizeof(wp))
	win.GetWindowPlacement(a.mw.Handle(), &wp)
	a.cfg.Maximized = wp.ShowCmd == win.SW_SHOWMAXIMIZED
	if !a.cfg.Maximized && !win.IsIconic(a.mw.Handle()) {
		a.cfg.WindowW = ws.Width * 96 / a.dpi
		a.cfg.WindowH = ws.Height * 96 / a.dpi
	}
	a.cfg.FromIP = a.ipFrom.IPString()
	a.cfg.ToIP = a.ipTo.IPString()
	cols := a.tv.Columns()
	for i := 0; i < cols.Len(); i++ {
		c := cols.At(i)
		if c.Visible() {
			a.cfg.ColumnWidths[c.Name()] = c.Width()
		}
	}
	a.cfg.save("")
}

// ---------- actions ----------

func (a *App) act(id, text, icon string, sc walk.Shortcut, fn func()) *walk.Action {
	ac := walk.NewAction()
	ac.SetText(text)
	if icon != "" {
		if b := toolIcon(icon, a.dpi); b != nil {
			ac.SetImage(b)
		}
	}
	if sc.Key != 0 {
		ac.SetShortcut(sc)
	}
	tip := strings.ReplaceAll(strings.TrimSuffix(text, "..."), "&", "")
	ac.SetToolTip(tip)
	if fn != nil {
		ac.Triggered().Attach(fn)
	}
	a.ac[id] = ac
	return ac
}

func (a *App) check(id, text string, checked bool, fn func(bool)) *walk.Action {
	ac := a.act(id, text, "", walk.Shortcut{}, nil)
	ac.SetCheckable(true)
	ac.SetChecked(checked)
	ac.Triggered().Attach(func() { fn(ac.Checked()) })
	return ac
}

func (a *App) createActions() {
	none := walk.Shortcut{}
	ctrl := func(k walk.Key) walk.Shortcut { return walk.Shortcut{Modifiers: walk.ModControl, Key: k} }
	// File
	a.act("newwin", T("New Window"), "", ctrl(walk.KeyN), func() {
		exe, _ := os.Executable()
		exec.Command(exe).Start()
	})
	a.act("scan", T("Start Scanning"), "", walk.Shortcut{Key: walk.KeyF5}, a.toggleScan)
	a.act("clear", T("Clear Results"), "clear", none, a.clearResults)
	a.act("load", T("Load Results..."), "load", ctrl(walk.KeyO), a.loadResultsDlg)
	a.act("save", T("Save Results..."), "save", ctrl(walk.KeyS), a.saveResultsDlg)
	a.act("import", T("Import IP/Host Name List..."), "", none, a.importList)
	a.act("saveconf", T("Save to File..."), "", none, a.saveConfigTo)
	a.act("loadconf", T("Load from File..."), "", none, a.loadConfigFrom)
	a.act("resetconf", T("Reset to Default"), "", none, a.resetConfig)
	a.act("exit", T("Exit"), "", walk.Shortcut{Modifiers: walk.ModAlt, Key: walk.KeyF4}, func() { a.mw.Close() })
	// View
	a.check("autoexpand", T("Auto Expand"), a.cfg.AutoExpand, func(b bool) { a.cfg.AutoExpand = b })
	a.act("collapse", T("Collapse All"), "collapse", none, func() { a.setExpandedAll(false) })
	a.act("expand", T("Expand All"), "expand", none, func() { a.setExpandedAll(true) })
	a.act("sharesonly", T("Shared Folders"), "shares", none, nil)
	a.ac["sharesonly"].SetCheckable(true)
	a.ac["sharesonly"].Triggered().Attach(func() {
		a.model.onlyShares = a.ac["sharesonly"].Checked()
		if a.model.onlyShares && !a.cfg.EnumShares {
			walk.MsgBox(a.mw, appName, T("Shared folder enumeration is disabled.\nEnable it in Options → Shares and rescan."), walk.MsgBoxIconInformation)
		}
		a.refresh()
		a.updateStatus()
	})
	a.act("filter", T("Setup Filter"), "filter", ctrl(walk.KeyL), a.showFilter)
	a.act("columns", T("Visible Columns..."), "", none, a.visibleColumnsDlg)
	a.check("gridlines", T("Grid Lines"), a.cfg.GridLines, func(b bool) { a.cfg.GridLines = b; a.applyViewSettings() })
	a.check("altrows", T("Alternating Row Colours"), a.cfg.AlternateRows, func(b bool) { a.cfg.AlternateRows = b; a.applyViewSettings() })
	a.check("highlightnew", T("Highlight New Devices"), a.cfg.HighlightNew, func(b bool) { a.cfg.HighlightNew = b; a.tv.Invalidate() })
	a.act("removesort", T("Remove Sorting"), "", none, func() { a.model.RemoveSorting() })
	a.act("legend", T("Legend"), "", none, a.legendDlg)
	a.act("highlight", T("Highlight Current Line"), "", none, func() { a.highlightSelected(true) })
	a.act("clearhighlight", T("Clear Highlight"), "", none, func() { a.highlightSelected(false) })
	// Actions
	a.act("rescanall", T("Rescan All Displayed Devices"), "refresh", none, func() { a.rescanList(false) })
	a.act("rescanoffline", T("Rescan Previously Undiscovered Devices"), "", none, func() { a.rescanList(true) })
	a.act("wol", T("Send Wake-On-LAN Signal"), "wol", none, a.wolSelected)
	a.act("savewol", T("Save MAC to WOL Manager"), "", none, a.saveMACToWOL)
	a.act("sendmsg", T("Send Message..."), "", none, a.sendMessageDlg)
	a.act("shutdown", T("Remote Shutdown..."), "", none, a.shutdownDlg)
	a.act("dhcp", T("DHCP Server Discovery..."), "devices", none, a.dhcpDiscovery)
	a.act("upnp", T("UPnP Device Discovery..."), "upnp", none, a.upnpDiscovery)
	a.act("mdns", T("mDNS Device Discovery..."), "", none, a.mdnsDiscovery)
	a.act("dupmac", T("Find Duplicate MAC Addresses..."), "", none, a.duplicateMACs)
	a.act("deleteinactive", T("Delete Inactive Devices"), "", none, a.deleteInactive)
	a.act("find", T("Find..."), "find", ctrl(walk.KeyF), a.showSearch)
	a.act("findnext", T("Find Next"), "", walk.Shortcut{Key: walk.KeyF3}, func() { a.search(1) })
	// Reports
	a.act("compare", T("Compare Scans..."), "", none, a.compareDlg)
	a.act("inventory", T("Device Summary..."), "", none, a.summaryDlg)
	// Options
	a.act("options", T("Program Options..."), "options", ctrl(walk.KeyP), a.optionsDlg)
	a.act("autodetect", T("Auto Detect Local IP Range"), "autodetect", ctrl(walk.KeyI), func() { a.autoDetect(false) })
	a.act("extip", T("Detect External IP Address"), "globe", none, a.detectExternalIP)
	a.act("fromhost", T("Detect IP Range from Host Name..."), "", none, a.rangeFromHost)
	a.act("paste", T("Paste IP from Clipboard"), "paste", walk.Shortcut{Modifiers: walk.ModControl | walk.ModShift, Key: walk.KeyV}, a.pasteIP)
	a.act("wolmgr", T("Wake-On-LAN Manager..."), "wol", none, a.wolManager)
	a.act("friendly", T("Friendly Names / Comments..."), "", none, a.friendlyNamesDlg)
	a.act("ignored", T("Ignored IP Addresses..."), "", none, a.ignoredDlg)
	a.act("ranges", T("Additional Ranges..."), "plus", none, a.extraRangesDlg)
	// Bookmarks
	a.act("addbookmark", T("Add Current Range..."), "", ctrl(walk.KeyD), a.addBookmark)
	a.act("managebookmarks", T("Manage Bookmarks..."), "", none, a.manageBookmarks)
	// Help
	a.act("help", T("Help"), "help", walk.Shortcut{Key: walk.KeyF1}, a.helpDlg)
	a.act("about", T("About..."), "about", none, a.aboutDlg)
	a.check("lang_de", "Deutsch", lang == "de", func(bool) { a.switchLanguage("de") })
	a.check("lang_en", "English", lang == "en", func(bool) { a.switchLanguage("en") })

	// context menu
	a.act("openhttp", T("As Web (HTTP)"), "web", none, func() { a.openSelected("http://%s/") })
	a.act("openhttps", T("As Secure Web (HTTPS)"), "", none, func() { a.openSelected("https://%s/") })
	a.act("openftp", T("As File Server (FTP)"), "", none, func() { a.openSelected("ftp://%s/") })
	a.act("openssh", T("As SSH..."), "cmd", none, a.openSSH)
	a.act("openexplorer", T("As Windows Share (Explorer)"), "folder", none, func() { a.openSelected(`\\%s`) })
	a.act("openrdp", T("As Remote Desktop"), "display", none, a.openRDP)
	a.act("copyip", T("IP Address"), "", none, func() { a.copyField("ip") })
	a.act("copymac", T("MAC Address"), "", none, func() { a.copyField("mac") })
	a.act("copyhost", T("Host Name"), "", none, func() { a.copyField("host") })
	a.act("copyrows", T("Selected Rows"), "", ctrl(walk.KeyC), func() { a.copyRows(false) })
	a.act("copyall", T("All Rows"), "", none, func() { a.copyRows(true) })
	a.act("props", T("Properties"), "", none, a.propertiesDlg)
	a.act("rescan", T("Rescan Device"), "refresh", none, a.rescanSelected)
	a.act("filtervalue", T("Setup Filter by This Value"), "filter", none, a.filterByCell)
	a.act("friendlyone", T("Assign Friendly Name..."), "", none, a.assignFriendly)
	a.act("delete", T("Delete from List"), "", walk.Shortcut{Key: walk.KeyDelete}, a.deleteSelected)
	a.act("shareexplore", T("Explore"), "folder", none, a.exploreShare)
	a.act("sharemap", T("Map Network Drive..."), "", none, a.mapShare)
	a.act("sharecopy", T("Copy Path"), "", none, a.copySharePath)
}

func (a *App) mkMenu(items ...interface{}) *walk.Menu {
	m, _ := walk.NewMenu()
	for _, it := range items {
		switch v := it.(type) {
		case string:
			if v == "-" {
				m.Actions().Add(walk.NewSeparatorAction())
			} else {
				m.Actions().Add(a.ac[v])
			}
		case *walk.Action:
			m.Actions().Add(v)
		}
	}
	return m
}

func (a *App) sub(text string, m *walk.Menu) *walk.Action {
	ac := walk.NewMenuAction(m)
	ac.SetText(text)
	return ac
}

func (a *App) buildMenus() {
	mm := a.mw.Menu()
	add := func(text string, m *walk.Menu) { mm.Actions().Add(a.sub(text, m)) }

	cfgMenu := a.mkMenu("saveconf", "loadconf", "resetconf")
	add(T("&File"), a.mkMenu("newwin", "scan", "-", "clear", "load", "save", "-", "import", a.sub(T("Current Config"), cfgMenu), "-", "exit"))
	treeMenu := a.mkMenu("autoexpand", "collapse", "expand")
	quick := a.mkMenu("sharesonly")
	add(T("&View"), a.mkMenu("gridlines", "altrows", "highlightnew", "-", a.sub(T("Quick Host Filter"), quick), "filter", "columns", "removesort", "legend",
		a.sub(T("Tree"), treeMenu), "-", "highlight", "clearhighlight"))
	add(T("&Actions"), a.mkMenu("rescanall", "rescanoffline", "-", "wol", "sendmsg", "shutdown", "-", "dhcp", "dupmac", "-", "upnp", "mdns", "-", "deleteinactive", "find"))
	add(T("&Reports"), a.mkMenu("compare", "-", "inventory"))
	ipMenu := a.mkMenu("autodetect", "extip", "fromhost")
	add(T("&Options"), a.mkMenu("options", "-", a.sub(T("IP Address"), ipMenu), "ranges", "paste", "wolmgr", "friendly", "ignored"))
	a.bookmarksMenu = a.mkMenu("-", "addbookmark", "managebookmarks")
	add(T("&Bookmarks"), a.bookmarksMenu)
	langMenu := a.mkMenu("lang_de", "lang_en")
	add(T("&Help"), a.mkMenu("help", "legend", "-", a.sub(T("Language"), langMenu), "-", "about"))
	a.refreshBookmarksMenu()

	openMenu := a.mkMenu("openhttp", "openhttps", "openftp", "openssh", "openexplorer", "openrdp")
	copyMenu := a.mkMenu("copyip", "copymac", "copyhost", "-", "copyrows", "copyall")
	wolMenu := a.mkMenu("wol", "savewol")
	a.hostMenu = a.mkMenu(a.sub(T("Open Device"), openMenu), a.sub(T("Copy"), copyMenu), "props", "rescan", "-", "filtervalue",
		"highlight", "clearhighlight", "-", a.sub(T("Wake-On-LAN"), wolMenu), "shutdown", "-", "friendlyone", "sendmsg", "delete")
	a.shareMenu = a.mkMenu("shareexplore", "sharemap", "sharecopy", "-", "props")
	a.tv.SetContextMenu(a.hostMenu)
	a.updateContextMenu()
}

func (a *App) buildToolbars() {
	if iml, err := walk.NewImageListForDPI(walk.SizeFrom96DPI(walk.Size{Width: 24, Height: 24}, a.dpi), 0, a.dpi); err == nil {
		a.tb.SetImageList(iml)
	}
	for _, id := range []string{"clear", "load", "save", "-", "collapse", "expand", "find", "paste", "-", "sharesonly", "filter", "-",
		"options", "dhcp", "wolmgr", "autodetect", "-", "upnp", "extip", "rescanall", "-", "help", "about"} {
		if id == "-" {
			a.tb.Actions().Add(walk.NewSeparatorAction())
		} else {
			a.tb.Actions().Add(a.ac[id])
		}
	}
	for i, ic := range []string{"plus", "autodetect", "paste", "bookmark"} {
		a.rIcons[i].SetImage(bitmapPx(ic, 16*a.dpi/96, a.dpi))
		a.rIcons[i].SetCursor(walk.CursorHand())
	}
}

func (a *App) refreshBookmarksMenu() {
	acts := a.bookmarksMenu.Actions()
	for _, ac := range a.bookmarkDynamic {
		acts.Remove(ac)
	}
	a.bookmarkDynamic = nil
	for i, b := range a.cfg.Bookmarks {
		b := b
		ac := walk.NewAction()
		label := b.Name
		if label == "" {
			label = b.From + " - " + b.To
		}
		ac.SetText(label)
		ac.Triggered().Attach(func() {
			a.ipFrom.SetIPString(b.From)
			a.ipTo.SetIPString(b.To)
			a.cfg.ExtraRanges = append([]string{}, b.Extra...)
			a.updateExtraLabel()
		})
		acts.Insert(i, ac)
		a.bookmarkDynamic = append(a.bookmarkDynamic, ac)
	}
}

func (a *App) applyViewSettings() {
	a.tv.SetAlternatingRowBG(a.cfg.AlternateRows)
	lv := a.tv.ListViewHandle()
	const lvsExGrid = 0x1
	if a.cfg.GridLines {
		win.SendMessage(lv, win.LVM_SETEXTENDEDLISTVIEWSTYLE, lvsExGrid, lvsExGrid)
	} else {
		win.SendMessage(lv, win.LVM_SETEXTENDEDLISTVIEWSTYLE, lvsExGrid, 0)
	}
	a.applyColumnVisibility()
	a.tv.Invalidate()
}

func (a *App) applyColumnVisibility() {
	cols := a.tv.Columns()
	for i, c := range allColumns {
		if i < cols.Len() {
			col := cols.At(i)
			v := c.Visible(a.cfg)
			if col.Visible() != v {
				col.SetVisible(v)
			}
		}
	}
}

// ---------- table interaction ----------

func (a *App) styleCell(st *walk.CellStyle) {
	rw, ok := a.model.Row(st.Row())
	if !ok {
		return
	}
	d := rw.d
	if st.Col() == 0 {
		switch rw.kind {
		case rowShare:
			if d.Shares[rw.idx].Type&0xff == stypePrinter {
				st.Image = smallIcon("printer", a.dpi)
			} else {
				st.Image = smallIcon("folder", a.dpi)
			}
		case rowPort:
			st.Image = smallIcon("port", a.dpi)
		default:
			name := "dot_on"
			if !d.Alive {
				name = "dot_off"
			} else if d.New && a.cfg.HighlightNew {
				name = "dot_new"
			}
			if a.model.hasChildren(d) && name != "dot_off" {
				if d.Expanded {
					name += "_c"
				} else {
					name += "_x"
				}
			}
			st.Image = smallIcon(name, a.dpi)
		}
	}
	if rw.kind != rowDevice {
		st.TextColor = walk.RGB(0x55, 0x55, 0x55)
		return
	}
	if !d.Alive {
		st.TextColor = walk.RGB(0x99, 0x99, 0x99)
	}
	if d.Highlighted {
		st.BackgroundColor = walk.RGB(0xff, 0xf3, 0xb0)
	}
	if d.DupMAC && allColumns[st.Col()].ID == "mac" {
		st.TextColor = walk.RGB(0xc6, 0x28, 0x28)
	}
}

func (a *App) currentRow() (row, bool) {
	return a.model.Row(a.tv.CurrentIndex())
}

func (a *App) selectedDevices() []*Device {
	var out []*Device
	seen := map[*Device]bool{}
	for _, i := range a.tv.SelectedIndexes() {
		if rw, ok := a.model.Row(i); ok && !seen[rw.d] {
			seen[rw.d] = true
			out = append(out, rw.d)
		}
	}
	if len(out) == 0 {
		if rw, ok := a.currentRow(); ok {
			out = append(out, rw.d)
		}
	}
	return out
}

func (a *App) toggleExpand(d *Device) {
	if !a.model.hasChildren(d) {
		return
	}
	d.Expanded = !d.Expanded
	a.refresh()
	if r := a.model.RowOf(d); r >= 0 {
		a.tv.SetCurrentIndex(r)
	}
}

func (a *App) setExpandedAll(b bool) {
	for _, d := range a.model.devices {
		d.Expanded = b
	}
	a.refresh()
}

func (a *App) onActivate() {
	rw, ok := a.currentRow()
	if !ok {
		return
	}
	switch rw.kind {
	case rowShare:
		a.exploreShare()
	case rowPort:
		p := rw.d.Ports[rw.idx]
		scheme := "http"
		if p == 443 || p == 8443 {
			scheme = "https"
		}
		shellOpen(fmt.Sprintf("%s://%s:%d/", scheme, rw.d.IP, p), "")
	default:
		if a.model.hasChildren(rw.d) {
			a.toggleExpand(rw.d)
		} else {
			a.propertiesDlg()
		}
	}
}

func (a *App) onTableKey(k walk.Key) {
	rw, ok := a.currentRow()
	if !ok {
		return
	}
	switch k {
	case walk.KeyRight, walk.KeyAdd:
		if rw.kind == rowDevice && !rw.d.Expanded {
			a.toggleExpand(rw.d)
		}
	case walk.KeyLeft, walk.KeySubtract:
		if rw.d.Expanded {
			a.toggleExpand(rw.d)
		}
	}
}

func (a *App) onTableMouse(x, y int, button walk.MouseButton) {
	idx := a.tv.IndexAt(x, y)
	if button == walk.LeftButton && idx >= 0 {
		if rw, ok := a.model.Row(idx); ok && rw.kind == rowDevice && x < 24*a.dpi/96 {
			a.toggleExpand(rw.d)
		}
	}
	a.lastClickCol = a.columnAt(x, y)
	if button == walk.RightButton && idx >= 0 {
		sel := a.tv.SelectedIndexes()
		in := false
		for _, s := range sel {
			if s == idx {
				in = true
			}
		}
		if !in {
			a.tv.SetSelectedIndexes([]int{idx})
			a.tv.SetCurrentIndex(idx)
		}
		a.updateContextMenu()
	}
}

func (a *App) updateContextMenu() {
	rw, ok := a.currentRow()
	if ok && rw.kind == rowShare {
		a.tv.SetContextMenu(a.shareMenu)
	} else {
		a.tv.SetContextMenu(a.hostMenu)
	}
	has := ok
	for _, id := range []string{"openhttp", "openhttps", "openftp", "openssh", "openexplorer", "openrdp", "copyip", "copymac", "copyhost",
		"copyrows", "props", "rescan", "filtervalue", "friendlyone", "delete", "wol", "savewol", "shutdown", "sendmsg", "highlight", "clearhighlight"} {
		a.ac[id].SetEnabled(has)
	}
}

func (a *App) highlightSelected(b bool) {
	for _, d := range a.selectedDevices() {
		d.Highlighted = b
	}
	a.tv.Invalidate()
}

// ---------- filter & search ----------

func (a *App) showFilter() {
	a.filterPanel.SetVisible(true)
	a.edFilterVal.SetFocus()
}

func (a *App) applyFilter() {
	col := a.cmbFilterCol.CurrentIndex()
	op := filterOps[0]
	if i := a.cmbFilterOp.CurrentIndex(); i >= 0 && i < len(filterOps) {
		op = filterOps[i]
	}
	a.model.filters = []Filter{{Col: col, Op: op, Value: a.edFilterVal.Text()}}
	a.refresh()
	a.updateStatus()
}

func (a *App) clearFilter() {
	a.model.filters = nil
	a.refresh()
	a.updateStatus()
}

func (a *App) filterByCell() {
	rw, ok := a.currentRow()
	if !ok {
		return
	}
	col := a.lastClickCol
	if col < 0 || col >= len(allColumns) {
		col = 0
	}
	a.cmbFilterCol.SetCurrentIndex(col)
	a.cmbFilterOp.SetCurrentIndex(2)
	a.edFilterVal.SetText(allColumns[col].Get(rw.d, a.cfg))
	a.showFilter()
}

type lvHitTestInfo struct {
	X, Y     int32
	Flags    uint32
	IItem    int32
	ISubItem int32
	IGroup   int32
}

// columnAt maps a point in table coordinates to a model column index.
func (a *App) columnAt(x, y int) int {
	ht := lvHitTestInfo{X: int32(x), Y: int32(y)}
	win.SendMessage(a.tv.ListViewHandle(), win.LVM_FIRST+57, 0, uintptr(unsafe.Pointer(&ht)))
	if ht.ISubItem < 0 {
		return 0
	}
	n := 0
	for i, c := range allColumns {
		if c.Visible(a.cfg) {
			if n == int(ht.ISubItem) {
				return i
			}
			n++
		}
	}
	return 0
}

func (a *App) showSearch() {
	a.searchPanel.SetVisible(true)
	a.edSearch.SetFocus()
	a.edSearch.SetTextSelection(0, -1)
}

func (a *App) search(dir int) {
	t := strings.ToLower(strings.TrimSpace(a.edSearch.Text()))
	if t == "" {
		a.showSearch()
		return
	}
	n := a.model.RowCount()
	if n == 0 {
		return
	}
	start := a.tv.CurrentIndex()
	for k := 1; k <= n; k++ {
		i := ((start+dir*k)%n + n) % n
		for c := range allColumns {
			if strings.Contains(strings.ToLower(fmt.Sprint(a.model.Value(i, c))), t) {
				a.tv.SetSelectedIndexes([]int{i})
				a.tv.SetCurrentIndex(i)
				a.tv.EnsureItemVisible(i)
				return
			}
		}
	}
	a.sbStatus.SetText(T("Not found: ") + t)
}

// ---------- open / copy ----------

func (a *App) openSelected(format string) {
	for _, d := range a.selectedDevices() {
		shellOpen(fmt.Sprintf(format, d.IP), "")
	}
}

func (a *App) openSSH() {
	ds := a.selectedDevices()
	if len(ds) == 0 {
		return
	}
	user, ok := inputBox(a.mw, T("SSH"), T("User name (leave empty for default):"), "")
	if !ok {
		return
	}
	for _, d := range ds {
		target := d.IP.String()
		if user != "" {
			target = user + "@" + target
		}
		cmd := exec.Command("cmd", "/c", "start", "SSH "+d.IP.String(), "ssh", target)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err != nil {
			walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
		}
	}
}

func (a *App) openRDP() {
	for _, d := range a.selectedDevices() {
		exec.Command("mstsc", "/v:"+d.IP.String()).Start()
	}
}

func (a *App) copyField(id string) {
	_, c := columnByID(id)
	var out []string
	for _, d := range a.selectedDevices() {
		out = append(out, c.Get(d, a.cfg))
	}
	walk.Clipboard().SetText(strings.Join(out, "\r\n"))
}

func (a *App) visibleCols() []*ColumnDef {
	var out []*ColumnDef
	for _, c := range allColumns {
		if c.Visible(a.cfg) {
			out = append(out, c)
		}
	}
	return out
}

func (a *App) copyRows(all bool) {
	cols := a.visibleCols()
	var lines []string
	var hdr []string
	for _, c := range cols {
		hdr = append(hdr, T(c.Title))
	}
	lines = append(lines, strings.Join(hdr, "\t"))
	devs := a.selectedDevices()
	if all {
		devs = a.model.visible
	}
	for _, d := range devs {
		var f []string
		for _, c := range cols {
			f = append(f, c.Get(d, a.cfg))
		}
		lines = append(lines, strings.Join(f, "\t"))
	}
	walk.Clipboard().SetText(strings.Join(lines, "\r\n"))
}

func (a *App) currentShare() (*Device, *Share) {
	rw, ok := a.currentRow()
	if !ok || rw.kind != rowShare {
		return nil, nil
	}
	return rw.d, &rw.d.Shares[rw.idx]
}

func (a *App) exploreShare() {
	d, s := a.currentShare()
	if s == nil {
		return
	}
	shellOpen(`\\`+d.IP.String()+`\`+s.Name, "")
}

func (a *App) copySharePath() {
	d, s := a.currentShare()
	if s == nil {
		return
	}
	walk.Clipboard().SetText(`\\` + d.IP.String() + `\` + s.Name)
}

func (a *App) mapShare() {
	d, s := a.currentShare()
	if s == nil {
		return
	}
	var free []string
	mask, _ := windowsLogicalDrives()
	for c := 'Z'; c >= 'D'; c-- {
		if mask&(1<<uint(c-'A')) == 0 {
			free = append(free, string(c)+":")
		}
	}
	drive, ok := comboBox(a.mw, T("Map Network Drive"), `Map \\`+d.IP.String()+`\`+s.Name+T(" to drive:"), free)
	if !ok || drive == "" {
		return
	}
	out, err := exec.Command("net", "use", drive, `\\`+d.IP.String()+`\`+s.Name, "/persistent:no").CombinedOutput()
	if err != nil {
		walk.MsgBox(a.mw, appName, strings.TrimSpace(string(out))+"\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	shellOpen(drive+`\`, "")
}

// ---------- range helpers ----------

func (a *App) updateExtraLabel() {
	n := len(a.cfg.ExtraRanges)
	if n == 0 {
		a.lblExtra.SetText("")
	} else {
		a.lblExtra.SetText(fmt.Sprintf(T("+%d more"), n))
	}
}

func (a *App) autoDetect(silent bool) {
	nets := localNetworks()
	if len(nets) == 0 {
		if !silent {
			walk.MsgBox(a.mw, appName, T("No active IPv4 network adapter found."), walk.MsgBoxIconWarning)
		}
		return
	}
	n := nets[0]
	if len(nets) > 1 && !silent {
		var labels []string
		for _, x := range nets {
			labels = append(labels, fmt.Sprintf("%s  –  %s  (%s - %s)", x.Name, x.Net.String(), x.First, x.Last))
		}
		i, ok := listChoice(a.mw, T("Auto Detect Local IP Range"), T("Select a network adapter:"), labels)
		if !ok {
			return
		}
		n = nets[i]
	}
	if ones, _ := n.Net.Mask.Size(); ones < 16 && !silent {
		if walk.MsgBox(a.mw, appName, fmt.Sprintf(T("The subnet %s is very large. Continue?"), n.Net), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
			return
		}
	}
	a.ipFrom.SetIP(n.First)
	a.ipTo.SetIP(n.Last)
}

var ipRangeRe = regexp.MustCompile(`(\d{1,3}(?:\.\d{1,3}){3})(?:\s*(?:-|–|to|\s)\s*(\d{1,3}(?:\.\d{1,3}){3}))?(?:/(\d{1,2}))?`)

func (a *App) pasteIP() {
	t, err := walk.Clipboard().Text()
	if err != nil || strings.TrimSpace(t) == "" {
		walk.MsgBox(a.mw, appName, T("The clipboard does not contain text."), walk.MsgBoxIconInformation)
		return
	}
	ms := ipRangeRe.FindAllStringSubmatch(t, -1)
	if len(ms) == 0 {
		walk.MsgBox(a.mw, appName, T("No IP address found in the clipboard."), walk.MsgBoxIconInformation)
		return
	}
	m := ms[0]
	if m[3] != "" {
		if r, err := parseRangeSpec(m[1] + "/" + m[3]); err == nil {
			a.ipFrom.SetIP(u32ToIP(r[0][0]))
			a.ipTo.SetIP(u32ToIP(r[0][1]))
		}
	} else if m[2] != "" {
		a.ipFrom.SetIPString(m[1])
		a.ipTo.SetIPString(m[2])
	} else if len(ms) >= 2 && ms[1][2] == "" && ms[1][3] == "" {
		a.ipFrom.SetIPString(m[1])
		a.ipTo.SetIPString(ms[1][1])
	} else {
		a.ipFrom.SetIPString(m[1])
		a.ipTo.SetIPString(m[1])
	}
}

func (a *App) rangeFromHost() {
	h, ok := inputBox(a.mw, T("Detect IP Range from Host Name"), T("Host name or IP address:"), "")
	if !ok || strings.TrimSpace(h) == "" {
		return
	}
	ips, err := net.LookupIP(strings.TrimSpace(h))
	var ip net.IP
	for _, x := range ips {
		if x.To4() != nil {
			ip = x.To4()
			break
		}
	}
	if err != nil || ip == nil {
		walk.MsgBox(a.mw, appName, T("Cannot resolve ")+h, walk.MsgBoxIconWarning)
		return
	}
	a.ipFrom.SetIP(net.IPv4(ip[0], ip[1], ip[2], 1))
	a.ipTo.SetIP(net.IPv4(ip[0], ip[1], ip[2], 254))
}

func (a *App) detectExternalIP() {
	a.sbStatus.SetText(T("Detecting external IP address..."))
	go func() {
		ip, err := externalIP()
		a.mw.Synchronize(func() {
			a.updateStatus()
			if err != nil {
				walk.MsgBox(a.mw, appName, T("Could not detect the external IP address:\n")+err.Error(), walk.MsgBoxIconWarning)
				return
			}
			host := ""
			if names, err := net.LookupAddr(ip); err == nil && len(names) > 0 {
				host = T("\nHost name: ") + strings.TrimSuffix(names[0], ".")
			}
			if walk.MsgBox(a.mw, T("External IP Address"), T("Your external IP address is ")+ip+host+T("\n\nCopy to clipboard?"), walk.MsgBoxYesNo|walk.MsgBoxIconInformation) == win.IDYES {
				walk.Clipboard().SetText(ip)
			}
		})
	}()
}

// ---------- results ----------

func (a *App) clearResults() {
	if a.scanner != nil {
		return
	}
	a.model.Clear()
	a.progress.SetValue(0)
	a.updateStatus()
	a.mw.SetTitle(appName + T(" – Network Scanner"))
}

func (a *App) saveResultsDlg() {
	if len(a.model.devices) == 0 {
		walk.MsgBox(a.mw, appName, T("There are no results to save."), walk.MsgBoxIconInformation)
		return
	}
	dlg := &walk.FileDialog{Title: T("Save Results"),
		Filter:   T("XML document (*.xml)|*.xml|JSON (*.json)|*.json|CSV - comma separated values (*.csv)|*.csv|HTML document (*.html)|*.html|Text file (*.txt)|*.txt"),
		FilePath: "scan-" + time.Now().Format("2006-01-02") + ".xml"}
	if ok, _ := dlg.ShowSave(a.mw); !ok {
		return
	}
	path := dlg.FilePath
	exts := []string{".xml", ".json", ".csv", ".html", ".txt"}
	if filepath.Ext(path) == "" && dlg.FilterIndex >= 1 && dlg.FilterIndex <= len(exts) {
		path += exts[dlg.FilterIndex-1]
	}
	if err := saveResults(path, a.model.visible, a.visibleCols(), a.cfg, a.lastRange); err != nil {
		walk.MsgBox(a.mw, appName, T("Cannot save: ")+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.cfg.LastFile = path
	a.sbStatus.SetText(T("Saved ") + filepath.Base(path))
}

func (a *App) loadResultsDlg() {
	dlg := &walk.FileDialog{Title: T("Load Results"), Filter: T("Scan results (*.xml;*.json)|*.xml;*.json|All files (*.*)|*.*")}
	if ok, _ := dlg.ShowOpen(a.mw); !ok {
		return
	}
	a.loadFile(dlg.FilePath)
}

func (a *App) loadFile(path string) {
	devs, rng, err := loadResults(path)
	if err != nil {
		walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
		return
	}
	a.model.Clear()
	for _, d := range devs {
		d.Expanded = a.cfg.AutoExpand
		a.model.Upsert(d)
	}
	a.lastRange = rng
	a.refresh()
	a.updateStatus()
	a.mw.SetTitle(appName + " – " + filepath.Base(path))
}

func (a *App) importList() {
	dlg := &walk.FileDialog{Title: T("Import IP/Host Name List"), Filter: T("Text files (*.txt;*.csv)|*.txt;*.csv|All files (*.*)|*.*")}
	if ok, _ := dlg.ShowOpen(a.mw); !ok {
		return
	}
	b, err := os.ReadFile(dlg.FilePath)
	if err != nil {
		walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
		return
	}
	var list []string
	for _, l := range strings.FieldsFunc(string(b), func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' || r == '\t' }) {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			list = append(list, l)
		}
	}
	a.cfg.ExtraRanges = list
	a.ipFrom.SetIPString("0.0.0.0")
	a.ipTo.SetIPString("0.0.0.0")
	a.updateExtraLabel()
	walk.MsgBox(a.mw, appName, fmt.Sprintf(T("%d entries imported. Click Start Scanning to scan them."), len(list)), walk.MsgBoxIconInformation)
}

func (a *App) saveConfigTo() {
	dlg := &walk.FileDialog{Title: T("Save Configuration"), Filter: T("Configuration (*.json)|*.json"), FilePath: "netzscan-config.json"}
	if ok, _ := dlg.ShowSave(a.mw); ok {
		a.cfg.FromIP, a.cfg.ToIP = a.ipFrom.IPString(), a.ipTo.IPString()
		if err := a.cfg.save(dlg.FilePath); err != nil {
			walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
		}
	}
}

func (a *App) loadConfigFrom() {
	dlg := &walk.FileDialog{Title: T("Load Configuration"), Filter: T("Configuration (*.json)|*.json")}
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		a.replaceConfig(loadSettings(dlg.FilePath))
	}
}

func (a *App) resetConfig() {
	if walk.MsgBox(a.mw, appName, T("Reset all settings to their default values?"), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == win.IDYES {
		n := defaultSettings()
		n.FriendlyNames, n.WOL, n.Bookmarks, n.KnownDevices = a.cfg.FriendlyNames, a.cfg.WOL, a.cfg.Bookmarks, a.cfg.KnownDevices
		a.replaceConfig(n)
	}
}

func (a *App) replaceConfig(n *Settings) {
	*a.cfg = *n
	a.ipFrom.SetIPString(a.cfg.FromIP)
	a.ipTo.SetIPString(a.cfg.ToIP)
	a.updateExtraLabel()
	a.refreshBookmarksMenu()
	a.applyViewSettings()
	a.refresh()
}

func (a *App) deleteSelected() {
	for _, d := range a.selectedDevices() {
		a.model.Remove(d)
	}
	a.refresh()
	a.updateStatus()
}

func (a *App) deleteInactive() {
	for _, d := range append([]*Device{}, a.model.devices...) {
		if !d.Alive {
			a.model.Remove(d)
		}
	}
	a.refresh()
	a.updateStatus()
}

func (a *App) duplicateMACs() {
	by := map[string][]*Device{}
	for _, d := range a.model.devices {
		if d.MAC != nil {
			k := formatMAC(d.MAC, 0)
			by[k] = append(by[k], d)
		}
	}
	var items []DiscoveryItem
	for _, d := range a.model.devices {
		d.DupMAC = false
	}
	for mac, ds := range by {
		if len(ds) > 1 {
			for _, d := range ds {
				d.DupMAC = true
				items = append(items, DiscoveryItem{Cols: []string{d.IP.String(), formatMAC(d.MAC, a.cfg.MACFormat), d.HostName, d.Vendor, mac}})
			}
		}
	}
	a.tv.Invalidate()
	sortItemsByIP(items)
	if len(items) == 0 {
		walk.MsgBox(a.mw, appName, T("No duplicate MAC addresses found in the current results.\n(Several IPs on one MAC are typical for routers, VMs or proxy ARP.)"), walk.MsgBoxIconInformation)
		return
	}
	resultTable(a.mw, T("Duplicate MAC Addresses"), []string{T("IP address"), T("MAC address"), T("Host name"), T("Vendor"), T("Group")}, items, nil)
}

type lvItem struct {
	Mask      uint32
	IItem     int32
	ISubItem  int32
	State     uint32
	StateMask uint32
	_         [64]byte
}

// refresh rebuilds the table, repaints it and keeps selection + focus.
func (a *App) refresh() {
	cur := a.model.ID(a.tv.CurrentIndex())
	sel := map[interface{}]bool{}
	for _, i := range a.tv.SelectedIndexes() {
		sel[a.model.ID(i)] = true
	}
	a.model.Refresh()
	lv := a.tv.ListViewHandle()
	const lvisFocused, lvisSelected = 0x1, 0x2
	for i := 0; i < a.model.RowCount() && (len(sel) > 0 || cur != nil); i++ {
		id := a.model.ID(i)
		var st uint32
		if sel[id] {
			st |= lvisSelected
		}
		if cur != nil && id == cur {
			st |= lvisFocused
		}
		if st != 0 {
			it := lvItem{State: st, StateMask: st}
			win.SendMessage(lv, win.LVM_SETITEMSTATE, uintptr(i), uintptr(unsafe.Pointer(&it)))
		}
	}
	win.SendMessage(lv, win.LVM_FIRST+21, 0, uintptr(a.model.RowCount())) // LVM_REDRAWITEMS
	win.RedrawWindow(lv, nil, 0, win.RDW_INVALIDATE|win.RDW_ERASE)
}

func trList(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = T(s)
	}
	return out
}

func (a *App) switchLanguage(l string) {
	a.ac["lang_de"].SetChecked(l == "de")
	a.ac["lang_en"].SetChecked(l == "en")
	if a.cfg.Language == l && lang == l {
		return
	}
	a.cfg.Language = l
	a.cfg.save("")
	msg := de["The language will change after restarting the program. Restart now?"]
	if l == "en" {
		msg = "The language will change after restarting the program. Restart now?"
	}
	if walk.MsgBox(a.mw, appName, msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == win.IDYES {
		if a.scanner != nil {
			a.scanner.Stop()
		}
		a.saveState()
		a.cfg.Language = l
		a.cfg.save("")
		if exe, err := os.Executable(); err == nil {
			exec.Command(exe).Start()
		}
		os.Exit(0)
	}
}
