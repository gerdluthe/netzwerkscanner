//go:build !windows

package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"image/color"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

//go:embed assets/icons/*_48.png assets/icons/app_64.png
var fyneIconFS embed.FS

// platformLanguageHook: nothing to do for Fyne (the UI is rebuilt on language change).
func platformLanguageHook() {}

type FApp struct {
	cfg *Settings
	a   fyne.App
	w   fyne.Window

	devices []*Device
	byIP    map[string]*Device
	view    []*Device
	cols    []*ColumnDef
	sortCol string
	sortAsc bool
	filter  string

	table      *widget.Table
	from, to   *widget.Entry
	btnScan    *widget.Button
	btnPause   *widget.Button
	lblStatus  *widget.Label
	lblThreads *widget.Label
	lblDevices *widget.Label
	lblTime    *widget.Label
	lblExtra   *widget.Label
	progress   *widget.ProgressBar

	scanner       *Scanner
	scanDone      chan struct{}
	firstEverScan bool
	rescanning    int32
	lastRange     string
	mobile        bool
}

var iconRes = map[string]fyne.Resource{}

func icon(name string) fyne.Resource {
	if r, ok := iconRes[name]; ok {
		return r
	}
	for _, f := range []string{"assets/icons/" + name + "_48.png", "assets/icons/" + name + "_64.png"} {
		if b, err := fyneIconFS.ReadFile(f); err == nil {
			r := fyne.NewStaticResource(name+".png", b)
			iconRes[name] = r
			return r
		}
	}
	return theme.QuestionIcon()
}

func runGUI(cfg *Settings) {
	a := app.NewWithID("de.netzscan.app")
	mobile := fyne.CurrentDevice().IsMobile()
	if mobile {
		// keep settings inside the app's private storage
		if root := a.Storage().RootURI(); root != nil {
			configDirOverride = root.Path()
			*cfg = *loadSettings("")
			setLanguage(cfg.Language)
		}
	}
	if v := a.Metadata().Version; v != "" && v != "0.0.1" {
		appVersion = v
	}
	a.SetIcon(icon("app"))
	a.Settings().SetTheme(lightTheme{})
	openURLHook = func(s string) error {
		u, err := url.Parse(s)
		if err != nil {
			return err
		}
		return a.OpenURL(u)
	}
	f := &FApp{cfg: cfg, a: a, byIP: map[string]*Device{}, sortCol: "ip", sortAsc: true, mobile: mobile}
	f.w = a.NewWindow(appName + T(" – Network Scanner"))
	f.w.SetContent(f.build())
	f.w.SetMainMenu(f.menu())
	f.w.Resize(fyne.NewSize(float32(max(cfg.WindowW, 760)), float32(max(cfg.WindowH, 520))))
	f.w.SetOnClosed(func() {
		if f.scanner != nil {
			f.scanner.Stop()
		}
		f.saveState()
	})
	if cfg.FromIP == "" || cfg.FromIP == "0.0.0.0" {
		f.autoDetect(true)
	}
	f.w.ShowAndRun()
}

func (f *FApp) saveState() {
	sz := f.w.Canvas().Size()
	if !f.mobile && sz.Width > 100 {
		f.cfg.WindowW, f.cfg.WindowH = int(sz.Width), int(sz.Height)
	}
	f.cfg.FromIP, f.cfg.ToIP = strings.TrimSpace(f.from.Text), strings.TrimSpace(f.to.Text)
	f.cfg.save("")
}

// ---------- layout ----------

func (f *FApp) build() fyne.CanvasObject {
	fromTxt, toTxt := f.cfg.FromIP, f.cfg.ToIP
	if f.from != nil {
		fromTxt, toTxt = f.from.Text, f.to.Text
	}
	if fromTxt == "0.0.0.0" {
		fromTxt, toTxt = "", ""
	}
	f.from = widget.NewEntry()
	f.from.SetText(fromTxt)
	f.from.SetPlaceHolder("192.168.1.1")
	f.to = widget.NewEntry()
	f.to.SetText(toTxt)
	f.to.SetPlaceHolder("192.168.1.254")
	f.from.OnSubmitted = func(string) { f.toggleScan() }
	f.to.OnSubmitted = func(string) { f.toggleScan() }

	f.btnScan = widget.NewButtonWithIcon(T("Start Scanning"), icon("play"), f.toggleScan)
	f.btnScan.Importance = widget.HighImportance
	f.btnPause = widget.NewButtonWithIcon("", icon("pause"), f.togglePause)
	f.btnPause.Disable()
	if f.scanner != nil {
		f.btnScan.SetText(T("Stop Scanning"))
		f.btnScan.SetIcon(icon("stop"))
		f.btnPause.Enable()
	}
	plus := widget.NewButtonWithIcon("", icon("plus"), f.extraRangesDlg)
	auto := widget.NewButtonWithIcon("", icon("autodetect"), func() { f.autoDetect(false) })
	f.lblExtra = widget.NewLabel("")
	f.updateExtraLabel()

	ipW := float32(150)
	fromBox := container.NewGridWrap(fyne.NewSize(ipW, f.from.MinSize().Height), f.from)
	toBox := container.NewGridWrap(fyne.NewSize(ipW, f.to.MinSize().Height), f.to)
	var rangeBar fyne.CanvasObject
	if f.mobile {
		rangeBar = container.NewVBox(
			container.NewBorder(nil, nil, widget.NewLabel(T("IPv4 From")), nil, f.from),
			container.NewBorder(nil, nil, widget.NewLabel(T("To")), nil, f.to),
			container.NewBorder(nil, nil, container.NewHBox(plus, auto), f.btnPause, f.btnScan),
		)
	} else {
		rangeBar = container.NewHBox(widget.NewLabel(T("IPv4 From")), fromBox, widget.NewLabel(T("To")), toBox,
			plus, auto, f.lblExtra, f.btnScan, f.btnPause, layout.NewSpacer())
	}

	tb := widget.NewToolbar(
		widget.NewToolbarAction(icon("clear"), f.clearResults),
		widget.NewToolbarAction(icon("load"), f.loadDlg),
		widget.NewToolbarAction(icon("save"), f.saveDlg),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(icon("options"), f.optionsDlg),
		widget.NewToolbarAction(icon("autodetect"), func() { f.autoDetect(false) }),
		widget.NewToolbarAction(icon("globe"), f.externalIP),
		widget.NewToolbarAction(icon("refresh"), f.rescanAll),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(icon("upnp"), f.upnpDlg),
		widget.NewToolbarAction(icon("devices"), f.mdnsDlg),
		widget.NewToolbarAction(icon("wol"), f.wolManager),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(icon("help"), f.helpDlg),
		widget.NewToolbarAction(icon("about"), f.aboutDlg),
	)

	search := widget.NewEntry()
	search.SetPlaceHolder(T("Filter results..."))
	search.SetText(f.filter)
	search.OnChanged = func(s string) {
		f.filter = strings.ToLower(strings.TrimSpace(s))
		f.refresh()
	}

	f.table = f.makeTable()

	f.lblStatus = widget.NewLabel(T("Ready"))
	f.lblThreads = widget.NewLabel(T("Threads: 0"))
	f.lblDevices = widget.NewLabel(fmt.Sprintf(T("Devices: %d / %d"), 0, 0))
	f.lblTime = widget.NewLabel("")
	f.progress = widget.NewProgressBar()
	f.progress.TextFormatter = func() string { return "" }
	var status fyne.CanvasObject
	if f.mobile {
		status = container.NewVBox(f.progress, container.NewHBox(f.lblStatus, layout.NewSpacer(), f.lblDevices))
	} else {
		status = container.NewVBox(f.progress, container.NewGridWithColumns(4, f.lblStatus, f.lblThreads, f.lblDevices, f.lblTime))
	}
	top := container.NewVBox(tb, rangeBar, search)
	f.refresh()
	return container.NewBorder(top, status, nil, nil, f.table)
}

func menuTitle(s string) string { return strings.ReplaceAll(T(s), "&", "") }

func (f *FApp) menu() *fyne.MainMenu {
	langItem := fyne.NewMenuItem(T("Language"), nil)
	de := fyne.NewMenuItem("Deutsch", func() { f.switchLanguage("de") })
	en := fyne.NewMenuItem("English", func() { f.switchLanguage("en") })
	de.Checked, en.Checked = lang == "de", lang == "en"
	langItem.ChildMenu = fyne.NewMenu("", de, en)
	return fyne.NewMainMenu(
		fyne.NewMenu(menuTitle("&File"),
			fyne.NewMenuItem(T("Clear Results"), f.clearResults),
			fyne.NewMenuItem(T("Load Results..."), f.loadDlg),
			fyne.NewMenuItem(T("Save Results..."), f.saveDlg),
		),
		fyne.NewMenu(menuTitle("&View"),
			fyne.NewMenuItem(T("Visible Columns..."), f.columnsDlg),
			fyne.NewMenuItem(T("Legend"), f.legendDlg),
		),
		fyne.NewMenu(menuTitle("&Actions"),
			fyne.NewMenuItem(T("Rescan All Displayed Devices"), f.rescanAll),
			fyne.NewMenuItem(T("Delete Inactive Devices"), f.deleteInactive),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(T("UPnP Device Discovery..."), f.upnpDlg),
			fyne.NewMenuItem(T("mDNS Device Discovery..."), f.mdnsDlg),
			fyne.NewMenuItem(T("DHCP Server Discovery..."), f.dhcpDlg),
		),
		fyne.NewMenu(menuTitle("&Reports"),
			fyne.NewMenuItem(T("Device Summary..."), f.summaryDlg),
		),
		fyne.NewMenu(menuTitle("&Options"),
			fyne.NewMenuItem(T("Program Options..."), f.optionsDlg),
			fyne.NewMenuItem(T("Auto Detect Local IP Range"), func() { f.autoDetect(false) }),
			fyne.NewMenuItem(T("Detect External IP Address"), f.externalIP),
			fyne.NewMenuItem(T("Additional Ranges..."), f.extraRangesDlg),
			fyne.NewMenuItem(T("Wake-On-LAN Manager..."), f.wolManager),
		),
		fyne.NewMenu(menuTitle("&Help"),
			fyne.NewMenuItem(T("Help"), f.helpDlg),
			langItem,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(T("About..."), f.aboutDlg),
		),
	)
}

func (f *FApp) switchLanguage(l string) {
	setLanguage(l)
	f.cfg.Language = l
	f.cfg.save("")
	f.w.SetTitle(appName + T(" – Network Scanner"))
	f.w.SetContent(f.build())
	f.w.SetMainMenu(f.menu())
}

// ---------- table ----------

type cellUI struct {
	widget.BaseWidget
	img *canvas.Image
	lbl *widget.Label
}

func newCell() *cellUI {
	c := &cellUI{img: canvas.NewImageFromResource(nil), lbl: widget.NewLabel("")}
	c.img.FillMode = canvas.ImageFillContain
	c.img.SetMinSize(fyne.NewSize(14, 14))
	c.lbl.Truncation = fyne.TextTruncateEllipsis
	c.ExtendBaseWidget(c)
	return c
}

func (c *cellUI) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, container.NewCenter(c.img), nil, c.lbl))
}

func (f *FApp) visibleColumns() []*ColumnDef {
	var out []*ColumnDef
	for _, c := range allColumns {
		if c.Visible(f.cfg) {
			out = append(out, c)
		}
	}
	return out
}

func (f *FApp) makeTable() *widget.Table {
	f.cols = f.visibleColumns()
	t := widget.NewTableWithHeaders(
		func() (int, int) { return len(f.view), len(f.cols) },
		func() fyne.CanvasObject { return newCell() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*cellUI)
			if id.Row >= len(f.view) || id.Col >= len(f.cols) {
				return
			}
			d := f.view[id.Row]
			c.lbl.SetText(f.cols[id.Col].Get(d, f.cfg))
			if !d.Alive {
				c.lbl.Importance = widget.LowImportance
			} else {
				c.lbl.Importance = widget.MediumImportance
			}
			c.lbl.Refresh()
			if id.Col == 0 {
				name := "dot_on"
				if !d.Alive {
					name = "dot_off"
				} else if d.New && f.cfg.HighlightNew {
					name = "dot_new"
				}
				c.img.Resource = icon(name)
				c.img.Show()
			} else {
				c.img.Hide()
			}
			c.img.Refresh()
		})
	t.ShowHeaderColumn = false
	t.CreateHeader = func() fyne.CanvasObject {
		b := widget.NewButton("", nil)
		b.Importance = widget.LowImportance
		b.Alignment = widget.ButtonAlignLeading
		return b
	}
	t.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		b := o.(*widget.Button)
		if id.Col < 0 || id.Col >= len(f.cols) {
			b.SetText("")
			return
		}
		c := f.cols[id.Col]
		txt := T(c.Title)
		if c.ID == f.sortCol {
			if f.sortAsc {
				txt += " ▲"
			} else {
				txt += " ▼"
			}
		}
		b.SetText(txt)
		b.OnTapped = func() {
			if f.sortCol == c.ID {
				f.sortAsc = !f.sortAsc
			} else {
				f.sortCol, f.sortAsc = c.ID, true
			}
			f.refresh()
		}
	}
	for i, c := range f.cols {
		w := float32(c.Width)
		if c.ID == "ip" {
			w += 24
		}
		t.SetColumnWidth(i, w)
	}
	t.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(f.view) {
			f.deviceDlg(f.view[id.Row])
		}
		t.UnselectAll()
	}
	return t
}

func (f *FApp) rebuildTable() { f.w.SetContent(f.build()) }

func (f *FApp) refresh() {
	f.view = f.view[:0]
	for _, d := range f.devices {
		if f.filter != "" {
			hit := false
			for _, c := range allColumns {
				if strings.Contains(strings.ToLower(c.Get(d, f.cfg)), f.filter) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		f.view = append(f.view, d)
	}
	_, col := columnByID(f.sortCol)
	if col == nil {
		_, col = columnByID("ip")
	}
	sort.SliceStable(f.view, func(i, j int) bool {
		a, b := f.view[i], f.view[j]
		var less, eq bool
		if col.Num != nil {
			x, y := col.Num(a), col.Num(b)
			less, eq = x < y, x == y
		} else {
			x, y := strings.ToLower(col.Get(a, f.cfg)), strings.ToLower(col.Get(b, f.cfg))
			if x == "" && y != "" {
				return false
			}
			if y == "" && x != "" {
				return true
			}
			less, eq = x < y, x == y
		}
		if eq {
			return ipToU32(a.IP) < ipToU32(b.IP)
		}
		if f.sortAsc {
			return less
		}
		return !less
	})
	if f.table != nil {
		f.table.Refresh()
	}
	f.updateStatus()
}

// ---------- scanning ----------

func (f *FApp) updateExtraLabel() {
	if f.lblExtra == nil {
		return
	}
	if n := len(f.cfg.ExtraRanges); n > 0 {
		f.lblExtra.SetText(fmt.Sprintf(T("+%d more"), n))
	} else {
		f.lblExtra.SetText("")
	}
}

func (f *FApp) toggleScan() {
	if f.scanner != nil {
		f.lblStatus.SetText(T("Stopping..."))
		f.scanner.Stop()
		return
	}
	f.startScan(nil)
}

func (f *FApp) startScan(list []net.IP) {
	if f.scanner != nil {
		return
	}
	cfg := f.cfg.clone()
	targets := list
	if targets == nil {
		from, to := strings.TrimSpace(f.from.Text), strings.TrimSpace(f.to.Text)
		if from == "" {
			from = "0.0.0.0"
		}
		if to == "" {
			to = from
		}
		var err error
		targets, err = buildTargets(from, to, f.cfg.ExtraRanges, f.cfg.Ignored, cfg.ScanRandom)
		if err != nil {
			dialog.ShowError(err, f.w)
			return
		}
		f.cfg.FromIP, f.cfg.ToIP = from, to
		f.lastRange = from + " - " + to
		f.devices = nil
		f.byIP = map[string]*Device{}
	}
	if fmt.Sprint(f.visibleColumns()) != fmt.Sprint(f.cols) {
		f.rebuildTable()
	}
	f.firstEverScan = len(f.cfg.KnownDevices) == 0
	sc := NewScanner(cfg, targets)
	f.scanner = sc
	sc.Start()
	f.btnScan.SetText(T("Stop Scanning"))
	f.btnScan.SetIcon(icon("stop"))
	f.btnPause.Enable()
	f.progress.SetValue(0)
	done := make(chan struct{})
	f.scanDone = done
	go func() {
		t := time.NewTicker(300 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				fyne.Do(func() { f.flush(sc) })
			case <-done:
				return
			}
		}
	}()
	f.refresh()
}

func (f *FApp) flush(sc *Scanner) {
	if sc != f.scanner {
		return
	}
	changed, closed := false, false
loop:
	for i := 0; i < 5000; i++ {
		select {
		case d, ok := <-sc.Results:
			if !ok {
				closed = true
				break loop
			}
			f.addResult(d)
			changed = true
		default:
			break loop
		}
	}
	if changed {
		f.refresh()
	} else {
		f.updateStatus()
	}
	if closed {
		f.finishScan(sc)
	}
}

func (f *FApp) addResult(d *Device) {
	key := d.IP.String()
	if d.MAC != nil {
		key = formatMAC(d.MAC, 0)
	}
	if d.Alive {
		if _, known := f.cfg.KnownDevices[key]; !known {
			if !f.firstEverScan {
				d.New = true
			}
			f.cfg.KnownDevices[key] = time.Now().Format("2006-01-02 15:04")
		}
	}
	if old, ok := f.byIP[d.Key()]; ok {
		if old.New {
			d.New = true
		}
		*old = *d
		return
	}
	f.byIP[d.Key()] = d
	f.devices = append(f.devices, d)
}

func (f *FApp) finishScan(sc *Scanner) {
	if f.scanDone != nil {
		close(f.scanDone)
		f.scanDone = nil
	}
	elapsed := time.Since(sc.Started)
	f.scanner = nil
	f.btnScan.SetText(T("Start Scanning"))
	f.btnScan.SetIcon(icon("play"))
	f.btnPause.Disable()
	f.btnPause.SetIcon(icon("pause"))
	f.progress.SetValue(1)
	f.refresh()
	f.lblTime.SetText(fmt.Sprintf(T("Scan time: %s"), elapsed.Round(100*time.Millisecond)))
	f.cfg.save("")
}

func (f *FApp) togglePause() {
	if f.scanner == nil {
		return
	}
	p := !f.scanner.Paused()
	f.scanner.SetPaused(p)
	if p {
		f.btnPause.SetIcon(icon("pause_on"))
	} else {
		f.btnPause.SetIcon(icon("pause"))
	}
	f.updateStatus()
}

func (f *FApp) updateStatus() {
	if f.lblDevices == nil {
		return
	}
	online := 0
	for _, d := range f.devices {
		if d.Alive {
			online++
		}
	}
	if len(f.view) != len(f.devices) {
		f.lblDevices.SetText(fmt.Sprintf(T("Devices: %d / %d (filtered: %d)"), online, len(f.devices), len(f.view)))
	} else {
		f.lblDevices.SetText(fmt.Sprintf(T("Devices: %d / %d"), online, len(f.devices)))
	}
	sc := f.scanner
	if sc == nil {
		if n := atomic.LoadInt32(&f.rescanning); n > 0 {
			f.lblStatus.SetText(fmt.Sprintf(T("Rescanning %d..."), n))
		} else {
			f.lblStatus.SetText(T("Ready"))
		}
		f.lblThreads.SetText(T("Threads: 0"))
		return
	}
	done, total := atomic.LoadInt64(&sc.Done), sc.Total
	pct := 0.0
	if total > 0 {
		pct = float64(done) / float64(total)
	}
	f.progress.SetValue(pct)
	switch {
	case sc.Paused():
		f.lblStatus.SetText(fmt.Sprintf(T("Paused (%d%%)"), int(pct*100)))
	case sc.ctx.Err() != nil:
		f.lblStatus.SetText(T("Stopping..."))
	default:
		f.lblStatus.SetText(fmt.Sprintf(T("Scanning... %d%%"), int(pct*100)))
	}
	f.lblThreads.SetText(fmt.Sprintf(T("Threads: %d"), atomic.LoadInt64(&sc.Active)))
	f.lblTime.SetText(fmt.Sprintf(T("%d of %d  ·  %s"), done, total, time.Since(sc.Started).Round(time.Second)))
}

func (f *FApp) rescanDevices(ds []*Device) {
	if len(ds) == 0 {
		return
	}
	if len(ds) > 8 {
		var ips []net.IP
		for _, d := range ds {
			ips = append(ips, d.IP)
		}
		f.startScan(ips)
		return
	}
	cfg := f.cfg.clone()
	var ports []int
	if cfg.CheckPorts {
		ports = parsePorts(cfg.Ports)
	}
	nets := localNetworks()
	for _, d := range ds {
		ip := d.IP
		atomic.AddInt32(&f.rescanning, 1)
		go func() {
			nd := ScanSingle(context.Background(), cfg, ip, ports, nets)
			fyne.Do(func() {
				atomic.AddInt32(&f.rescanning, -1)
				if nd == nil {
					nd = &Device{IP: ip, RTT: -1, LastScan: time.Now()}
				}
				f.addResult(nd)
				f.refresh()
			})
		}()
	}
	f.updateStatus()
}

func (f *FApp) rescanAll() { f.rescanDevices(append([]*Device{}, f.view...)) }

func (f *FApp) clearResults() {
	if f.scanner != nil {
		return
	}
	f.devices = nil
	f.byIP = map[string]*Device{}
	f.progress.SetValue(0)
	f.lblTime.SetText("")
	f.refresh()
}

func (f *FApp) deleteInactive() {
	var keep []*Device
	for _, d := range f.devices {
		if d.Alive {
			keep = append(keep, d)
		} else {
			delete(f.byIP, d.Key())
		}
	}
	f.devices = keep
	f.refresh()
}

func (f *FApp) autoDetect(silent bool) {
	nets := localNetworks()
	if len(nets) == 0 {
		if !silent {
			dialog.ShowInformation(appName, T("No active IPv4 network adapter found."), f.w)
		}
		return
	}
	apply := func(n LocalNet) {
		f.from.SetText(n.First.String())
		f.to.SetText(n.Last.String())
	}
	if len(nets) == 1 || silent {
		apply(nets[0])
		return
	}
	var labels []string
	for _, n := range nets {
		labels = append(labels, fmt.Sprintf("%s – %s (%s - %s)", n.Name, n.Net, n.First, n.Last))
	}
	sel := widget.NewRadioGroup(labels, nil)
	sel.SetSelected(labels[0])
	dialog.ShowCustomConfirm(T("Auto Detect Local IP Range"), T("OK"), T("Cancel"),
		container.NewVBox(widget.NewLabel(T("Select a network adapter:")), sel), func(ok bool) {
			if !ok {
				return
			}
			for i, l := range labels {
				if l == sel.Selected {
					apply(nets[i])
				}
			}
		}, f.w)
}

func (f *FApp) externalIP() {
	f.lblStatus.SetText(T("Detecting external IP address..."))
	go func() {
		ip, err := externalIP()
		fyne.Do(func() {
			f.updateStatus()
			if err != nil {
				dialog.ShowError(fmt.Errorf("%s%v", T("Could not detect the external IP address:\n"), err), f.w)
				return
			}
			dialog.ShowConfirm(T("External IP Address"), T("Your external IP address is ")+ip+T("\n\nCopy to clipboard?"), func(ok bool) {
				if ok {
					f.a.Clipboard().SetContent(ip)
				}
			}, f.w)
		})
	}()
}

// ---------- dialogs ----------

func (f *FApp) bigDialog(title string, content fyne.CanvasObject, buttons ...fyne.CanvasObject) dialog.Dialog {
	var d dialog.Dialog
	closeBtn := widget.NewButton(T("Close"), func() { d.Hide() })
	row := container.NewHBox(append(append([]fyne.CanvasObject{}, buttons...), layout.NewSpacer(), closeBtn)...)
	d = dialog.NewCustomWithoutButtons(title, container.NewBorder(nil, row, nil, nil, content), f.w)
	sz := f.w.Canvas().Size()
	d.Resize(fyne.NewSize(min(sz.Width*0.95, 860), min(sz.Height*0.9, 560)))
	d.Show()
	return d
}

func (f *FApp) deviceDlg(d *Device) {
	var b strings.Builder
	add := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%-18s %s\n", T(k)+":", v)
		}
	}
	add("IP address", d.IP.String())
	if d.Alive {
		add("Status", T("Online"))
	} else {
		add("Status", T("No response"))
	}
	add("Detected by", d.Method)
	if d.Alive && d.RTT >= 0 {
		add("Response time", fmt.Sprintf("%d ms", d.RTT))
	}
	if d.TTL > 0 {
		add("TTL", strconv.Itoa(d.TTL))
	}
	add("Host name", d.HostName)
	add("NetBIOS name", d.NetBIOS)
	add("Workgroup", d.Workgroup)
	add("MAC address", formatMAC(d.MAC, f.cfg.MACFormat))
	add("MAC vendor", d.Vendor)
	add("Device type", d.DeviceType)
	add("Comment", friendlyName(d, f.cfg))
	if len(d.Ports) > 0 {
		var ps []string
		for _, p := range d.Ports {
			if n := serviceName(p); n != "" {
				ps = append(ps, fmt.Sprintf("%d (%s)", p, n))
			} else {
				ps = append(ps, strconv.Itoa(p))
			}
		}
		add("Open TCP ports", strings.Join(ps, ", "))
	}
	add("HTTP server", d.HTTPServer)
	add("Web page title", d.HTTPTitle)
	if !d.LastScan.IsZero() {
		add("Last scan", d.LastScan.Format("2006-01-02 15:04:05"))
	}
	txt := widget.NewLabel(b.String())
	txt.TextStyle = fyne.TextStyle{Monospace: true}
	txt.Wrapping = fyne.TextWrapWord

	var dlg dialog.Dialog
	ipS := d.IP.String()
	acts := []fyne.CanvasObject{
		widget.NewButtonWithIcon("HTTP", icon("web"), func() { shellOpen("http://"+ipS+"/", "") }),
		widget.NewButton("HTTPS", func() { shellOpen("https://"+ipS+"/", "") }),
		widget.NewButtonWithIcon(T("Copy"), theme.ContentCopyIcon(), func() { f.a.Clipboard().SetContent(b.String()) }),
		widget.NewButtonWithIcon(T("Rescan Device"), icon("refresh"), func() { f.rescanDevices([]*Device{d}); dlg.Hide() }),
		widget.NewButtonWithIcon("WOL", icon("wol"), func() {
			if d.MAC == nil {
				dialog.ShowInformation("Wake-On-LAN", ipS+T(": MAC address unknown"), f.w)
				return
			}
			if err := sendWOL(d.MAC); err != nil {
				dialog.ShowError(err, f.w)
			} else {
				dialog.ShowInformation("Wake-On-LAN", fmt.Sprintf(T("Wake-On-LAN signal sent to %d device(s)."), 1), f.w)
			}
		}),
		widget.NewButton(T("Assign Friendly Name..."), func() { dlg.Hide(); f.assignFriendly(d) }),
		widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			delete(f.byIP, d.Key())
			for i, x := range f.devices {
				if x == d {
					f.devices = append(f.devices[:i], f.devices[i+1:]...)
					break
				}
			}
			f.refresh()
			dlg.Hide()
		}),
	}
	var bar fyne.CanvasObject = container.NewHBox(acts...)
	if f.mobile {
		bar = container.NewGridWithColumns(3, acts...)
	}
	dlg = f.bigDialog(T("Properties – ")+ipS, container.NewBorder(nil, bar, nil, nil, container.NewVScroll(txt)))
}

func (f *FApp) assignFriendly(d *Device) {
	e := widget.NewEntry()
	e.SetText(friendlyName(d, f.cfg))
	dialog.ShowForm(T("Assign Friendly Name"), T("OK"), T("Cancel"), []*widget.FormItem{widget.NewFormItem(T("Name"), e)}, func(ok bool) {
		if !ok {
			return
		}
		k := d.IP.String()
		if d.MAC != nil {
			k = formatMAC(d.MAC, 0)
		}
		if strings.TrimSpace(e.Text) == "" {
			delete(f.cfg.FriendlyNames, k)
		} else {
			f.cfg.FriendlyNames[k] = strings.TrimSpace(e.Text)
		}
		f.cfg.save("")
		f.rebuildTable()
	}, f.w)
}

func (f *FApp) textListDlg(title, prompt string, lines []string, onOK func([]string)) {
	e := widget.NewMultiLineEntry()
	e.SetText(strings.Join(lines, "\n"))
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.SetMinRowsVisible(8)
	d := dialog.NewCustomConfirm(title, T("OK"), T("Cancel"), container.NewBorder(widget.NewLabel(strings.ReplaceAll(prompt, "\r", "")), nil, nil, nil, e), func(ok bool) {
		if !ok {
			return
		}
		var out []string
		for _, l := range strings.Split(e.Text, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
		onOK(out)
	}, f.w)
	d.Resize(fyne.NewSize(640, 420))
	d.Show()
}

func (f *FApp) extraRangesDlg() {
	f.textListDlg(T("Additional Ranges"), T("One entry per line. Supported formats:\r\n  192.168.2.1-192.168.2.254   ·   10.0.0.0/24   ·   192.168.5.10   ·   host name\r\nThese are scanned together with the From/To range."),
		f.cfg.ExtraRanges, func(l []string) {
			for _, x := range l {
				if _, err := parseRangeSpec(x); err != nil {
					dialog.ShowError(err, f.w)
					return
				}
			}
			f.cfg.ExtraRanges = l
			f.updateExtraLabel()
			f.cfg.save("")
		})
}

func (f *FApp) columnsDlg() {
	var checks []*widget.Check
	box := container.NewVBox()
	for _, c := range allColumns {
		title := T(c.Title)
		if !c.Default && c.Auto != nil {
			title += T("  (when enabled in options)")
		}
		ch := widget.NewCheck(title, nil)
		ch.SetChecked(!f.cfg.HiddenColumns[c.ID])
		if c.ID == "ip" {
			ch.Disable()
		}
		checks = append(checks, ch)
		box.Add(ch)
	}
	d := dialog.NewCustomConfirm(T("Visible Columns"), T("OK"), T("Cancel"), container.NewVScroll(box), func(ok bool) {
		if !ok {
			return
		}
		for i, c := range allColumns {
			if c.ID == "ip" {
				continue
			}
			if checks[i].Checked {
				delete(f.cfg.HiddenColumns, c.ID)
			} else {
				f.cfg.HiddenColumns[c.ID] = true
			}
		}
		f.cfg.save("")
		f.rebuildTable()
	}, f.w)
	d.Resize(fyne.NewSize(460, 520))
	d.Show()
}

func (f *FApp) legendDlg() {
	rows := container.NewVBox()
	for _, e := range [][2]string{
		{"dot_on", "Device is online (responded to ping, ARP or TCP)"},
		{"dot_new", "New device – seen for the first time"},
		{"dot_off", "Device did not respond (shown when 'Display inactive devices' is on)"},
	} {
		img := canvas.NewImageFromResource(icon(e[0]))
		img.SetMinSize(fyne.NewSize(16, 16))
		img.FillMode = canvas.ImageFillContain
		rows.Add(container.NewHBox(img, widget.NewLabel(T(e[1]))))
	}
	dialog.ShowCustom(T("Legend"), T("Close"), rows, f.w)
}

func (f *FApp) aboutDlg() {
	img := canvas.NewImageFromResource(icon("app"))
	img.SetMinSize(fyne.NewSize(64, 64))
	img.FillMode = canvas.ImageFillContain
	title := widget.NewLabelWithStyle(appName+T(" Network Scanner"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := widget.NewLabel(T("Version ") + appVersion + "\n" + T("Fast multi-threaded IPv4 scanner: ping, ARP, DNS/NetBIOS,\nMAC vendors, TCP ports, SMB shares, Wake-on-LAN,\nUPnP / mDNS / DHCP discovery.") + "\n\n" +
		T("Icons: Icons8 Flat Color Icons and Microsoft Fluent UI (MIT).\nMAC vendor data: IEEE OUI registry."))
	dialog.ShowCustom(T("About ")+appName, T("OK"), container.NewHBox(container.NewVBox(img), container.NewVBox(title, body)), f.w)
}

func (f *FApp) helpDlg() {
	l := widget.NewLabel(helpText())
	l.TextStyle = fyne.TextStyle{Monospace: true}
	if f.mobile {
		l.Wrapping = fyne.TextWrapWord
	}
	f.bigDialog(appName+T(" Help"), container.NewScroll(l))
}

// ---------- options ----------

func numEntry(v int) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(strconv.Itoa(v))
	return e
}

func (f *FApp) optionsDlg() {
	c := f.cfg
	threads, timeout, attempts := numEntry(c.MaxThreads), numEntry(c.PingTimeout), numEntry(c.Attempts)
	method := widget.NewSelect([]string{T("ICMP only"), T("ARP only (local subnet)"), T("Both ICMP and ARP")}, nil)
	method.SetSelectedIndex(c.PingMethod)
	chk := func(text string, v bool) *widget.Check {
		ch := widget.NewCheck(T(text), nil)
		ch.SetChecked(v)
		return ch
	}
	cAlways := chk("Always analyse device (even if ping failed)", c.AlwaysAnalyse)
	cMethod := chk("Show successful ping method", c.ShowPingMethod)
	cInactive := chk("Display inactive devices", c.DisplayInactive)
	cTTL := chk("Display Time To Live (TTL)", c.DisplayTTL)
	cRandom := chk("Scan in random order", c.ScanRandom)
	cLast := chk("Show last-scan time", c.ShowLastScan)
	cNew := chk("Highlight devices seen for the first time", c.HighlightNew)

	cHost := chk("Resolve host names", c.ResolveHostNames)
	hostM := widget.NewSelect([]string{T("DNS, then NetBIOS"), T("DNS only"), T("NetBIOS only")}, nil)
	hostM.SetSelectedIndex(c.HostNameMethod)
	cMAC := chk("Resolve MAC addresses (ARP / NetBIOS)", c.ResolveMAC)
	cVendor := chk("Look up network card vendor", c.LookupVendor)
	cType := chk("Recognise device type (offline heuristics)", c.DeviceTypeGuess)
	cHTTP := chk("Grab HTTP server banner and page title", c.GrabHTTP)
	cWG := chk("Look up LAN group/domain (NetBIOS)", c.LookupWorkgroup)

	cPorts := chk("Check for open TCP ports (one or more, comma- or dash-separated)", c.CheckPorts)
	ports := widget.NewEntry()
	ports.SetText(c.Ports)
	cSvc := chk("Display service names rather than port numbers", c.ServiceNames)

	macF := widget.NewSelect([]string{"AA-BB-CC-DD-EE-FF", "AA:BB:CC:DD:EE:FF", "aabbccddeeff", "aa:bb:cc:dd:ee:ff"}, nil)
	macF.SetSelectedIndex(c.MACFormat)
	delim := widget.NewSelect([]string{";", ",", "Tab"}, nil)
	if c.CSVDelim == "\t" {
		delim.SetSelected("Tab")
	} else {
		delim.SetSelected(c.CSVDelim)
	}
	langSel := widget.NewSelect([]string{"Deutsch", "English"}, nil)
	if lang == "en" {
		langSel.SetSelectedIndex(1)
	} else {
		langSel.SetSelectedIndex(0)
	}

	cols := 2
	if f.mobile {
		cols = 1
	}
	general := container.NewVBox(
		widget.NewForm(widget.NewFormItem(T("Max. threads"), threads), widget.NewFormItem(T("Ping timeout, ms"), timeout),
			widget.NewFormItem(T("Attempts"), attempts), widget.NewFormItem(T("Method"), method)),
		container.NewGridWithColumns(cols, cAlways, cMethod, cInactive, cTTL, cRandom, cLast, cNew))
	additional := container.NewGridWithColumns(cols, cHost, hostM, cMAC, cVendor, cType, cHTTP, cWG)
	portTab := container.NewVBox(cPorts, ports,
		widget.NewLabel(T("Examples:  80,443   ·   21-23,80,443,445,3389,8080   ·   1-1024")), cSvc)
	view := widget.NewForm(widget.NewFormItem(T("Language"), langSel), widget.NewFormItem(T("MAC address format"), macF),
		widget.NewFormItem(T("CSV file delimiter"), delim))
	tabs := container.NewAppTabs(
		container.NewTabItem(T("General"), container.NewVScroll(general)),
		container.NewTabItem(T("Additional"), container.NewVScroll(additional)),
		container.NewTabItem(T("Ports"), container.NewVScroll(portTab)),
		container.NewTabItem(T("View"), container.NewVScroll(view)),
	)
	d := dialog.NewCustomConfirm(T("Options"), T("OK"), T("Cancel"), tabs, func(ok bool) {
		if !ok {
			return
		}
		atoi := func(e *widget.Entry, def, lo, hi int) int {
			v, err := strconv.Atoi(strings.TrimSpace(e.Text))
			if err != nil {
				return def
			}
			return max(lo, min(hi, v))
		}
		settingsMu.Lock()
		c.MaxThreads = atoi(threads, c.MaxThreads, 1, 1024)
		c.PingTimeout = atoi(timeout, c.PingTimeout, 50, 30000)
		c.Attempts = atoi(attempts, c.Attempts, 1, 10)
		c.PingMethod = method.SelectedIndex()
		c.AlwaysAnalyse, c.ShowPingMethod, c.DisplayInactive, c.DisplayTTL = cAlways.Checked, cMethod.Checked, cInactive.Checked, cTTL.Checked
		c.ScanRandom, c.ShowLastScan, c.HighlightNew = cRandom.Checked, cLast.Checked, cNew.Checked
		c.ResolveHostNames, c.HostNameMethod, c.ResolveMAC = cHost.Checked, hostM.SelectedIndex(), cMAC.Checked
		c.LookupVendor, c.DeviceTypeGuess, c.GrabHTTP, c.LookupWorkgroup = cVendor.Checked, cType.Checked, cHTTP.Checked, cWG.Checked
		c.CheckPorts, c.Ports, c.ServiceNames = cPorts.Checked, ports.Text, cSvc.Checked
		c.MACFormat = macF.SelectedIndex()
		if delim.Selected == "Tab" {
			c.CSVDelim = "\t"
		} else if delim.Selected != "" {
			c.CSVDelim = delim.Selected
		}
		settingsMu.Unlock()
		c.save("")
		newLang := "de"
		if langSel.SelectedIndex() == 1 {
			newLang = "en"
		}
		if newLang != lang {
			f.switchLanguage(newLang)
		} else {
			f.rebuildTable()
		}
	}, f.w)
	sz := f.w.Canvas().Size()
	d.Resize(fyne.NewSize(min(sz.Width*0.95, 780), min(sz.Height*0.9, 540)))
	d.Show()
}

// ---------- files ----------

func (f *FApp) saveDlg() {
	if len(f.devices) == 0 {
		dialog.ShowInformation(appName, T("There are no results to save."), f.w)
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()
		ext := strings.ToLower(w.URI().Extension())
		if ext == "" {
			ext = ".csv"
		}
		var buf bytes.Buffer
		if err := saveResultsTo(&buf, ext, f.view, f.visibleColumns(), f.cfg, f.lastRange); err != nil {
			dialog.ShowError(err, f.w)
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			dialog.ShowError(err, f.w)
			return
		}
		f.lblStatus.SetText(T("Saved ") + w.URI().Name())
	}, f.w)
	d.SetFileName("scan-" + time.Now().Format("2006-01-02") + ".csv")
	d.Show()
}

func (f *FApp) loadDlg() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		devs, rng, err := loadResultsFrom(r, strings.ToLower(r.URI().Extension()))
		if err != nil {
			dialog.ShowError(err, f.w)
			return
		}
		f.devices = nil
		f.byIP = map[string]*Device{}
		for _, d := range devs {
			f.byIP[d.Key()] = d
			f.devices = append(f.devices, d)
		}
		f.lastRange = rng
		f.refresh()
		f.w.SetTitle(appName + " – " + filepath.Base(r.URI().Name()))
	}, f.w)
	if !f.mobile {
		d.SetFilter(storage.NewExtensionFileFilter([]string{".xml", ".json"}))
	}
	d.Show()
}

// ---------- discovery & reports ----------

func (f *FApp) resultTable(title string, cols []string, items []DiscoveryItem, refresh func() ([]DiscoveryItem, error)) {
	data := items
	lbl := widget.NewLabel("")
	t := widget.NewTableWithHeaders(
		func() (int, int) { return len(data), len(cols) },
		func() fyne.CanvasObject { l := widget.NewLabel(""); l.Truncation = fyne.TextTruncateEllipsis; return l },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row < len(data) && id.Col < len(data[id.Row].Cols) {
				l.SetText(data[id.Row].Cols[id.Col])
			} else {
				l.SetText("")
			}
		})
	t.ShowHeaderColumn = false
	t.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok && id.Col >= 0 && id.Col < len(cols) {
			l.SetText(T(cols[id.Col]))
		}
	}
	for i := range cols {
		t.SetColumnWidth(i, 140)
	}
	t.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(data) && len(data[id.Row].Cols) > 0 && net.ParseIP(data[id.Row].Cols[0]) != nil {
			shellOpen("http://"+data[id.Row].Cols[0]+"/", "")
		}
		t.UnselectAll()
	}
	copyAll := widget.NewButtonWithIcon(T("Copy All"), theme.ContentCopyIcon(), func() {
		lines := []string{strings.Join(cols, "\t")}
		for _, it := range data {
			lines = append(lines, strings.Join(it.Cols, "\t"))
		}
		f.a.Clipboard().SetContent(strings.Join(lines, "\n"))
	})
	btns := []fyne.CanvasObject{lbl, copyAll}
	var run func()
	if refresh != nil {
		var rb *widget.Button
		rb = widget.NewButtonWithIcon(T("Refresh"), theme.ViewRefreshIcon(), func() { run() })
		run = func() {
			rb.Disable()
			lbl.SetText(T("Searching..."))
			go func() {
				it, err := refresh()
				fyne.Do(func() {
					rb.Enable()
					if err != nil {
						lbl.SetText(T("Error: ") + err.Error())
						return
					}
					data = it
					t.Refresh()
					lbl.SetText(fmt.Sprintf(T("%d item(s) found"), len(it)))
				})
			}()
		}
		btns = append(btns, rb)
	} else {
		lbl.SetText(fmt.Sprintf(T("%d item(s)"), len(items)))
	}
	f.bigDialog(T(title), t, btns...)
	if run != nil && items == nil {
		run()
	}
}

func (f *FApp) upnpDlg() {
	f.resultTable("UPnP Device Discovery", []string{"IP address", "Name", "Manufacturer", "Model", "Device type", "Server", "Location"}, nil,
		func() ([]DiscoveryItem, error) { return discoverUPnP(4 * time.Second) })
}

func (f *FApp) mdnsDlg() {
	f.resultTable("mDNS Device Discovery", []string{"IP address", "Instance", "Service", "Host", "Port", "TXT"}, nil,
		func() ([]DiscoveryItem, error) { return discoverMDNS(4 * time.Second) })
}

func (f *FApp) dhcpDlg() {
	f.resultTable("DHCP Server Discovery", []string{"DHCP server", "Offered IP", "Subnet mask", "Router", "DNS servers", "Domain", "Lease time"}, nil,
		func() ([]DiscoveryItem, error) { return discoverDHCP(5 * time.Second) })
}

func (f *FApp) summaryDlg() {
	byType, byVendor := map[string]int{}, map[string]int{}
	online := 0
	for _, d := range f.devices {
		if !d.Alive {
			continue
		}
		online++
		t, v := d.DeviceType, d.Vendor
		if t == "" {
			t = T("(unknown)")
		}
		if v == "" {
			v = T("(unknown)")
		}
		byType[t]++
		byVendor[v]++
	}
	items := []DiscoveryItem{{Cols: []string{T("Total"), T("Online devices"), strconv.Itoa(online)}}}
	for _, grp := range []struct {
		name string
		m    map[string]int
	}{{T("Device type"), byType}, {T("Vendor"), byVendor}} {
		var keys []string
		for k := range grp.m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return grp.m[keys[i]] > grp.m[keys[j]] })
		for _, k := range keys {
			items = append(items, DiscoveryItem{Cols: []string{grp.name, k, strconv.Itoa(grp.m[k])}})
		}
	}
	f.resultTable("Device Summary", []string{"Category", "Value", "Count"}, items, nil)
}

func (f *FApp) wolManager() {
	sel := -1
	list := widget.NewList(func() int { return len(f.cfg.WOL) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			e := f.cfg.WOL[i]
			o.(*widget.Label).SetText(strings.TrimSpace(e.Name + "   " + e.MAC + "   " + e.IP))
		})
	list.OnSelected = func(i widget.ListItemID) { sel = i }
	name, mac := widget.NewEntry(), widget.NewEntry()
	name.SetPlaceHolder(T("Name"))
	mac.SetPlaceHolder("AA-BB-CC-DD-EE-FF")
	add := widget.NewButtonWithIcon(T("Add"), theme.ContentAddIcon(), func() {
		hw, err := parseMAC(mac.Text)
		if err != nil {
			dialog.ShowError(err, f.w)
			return
		}
		f.cfg.WOL = append(f.cfg.WOL, WOLEntry{Name: name.Text, MAC: formatMAC(hw, 0)})
		f.cfg.save("")
		list.Refresh()
	})
	wake := widget.NewButtonWithIcon(T("Wake Up Selected"), icon("wol"), func() {
		if sel < 0 || sel >= len(f.cfg.WOL) {
			return
		}
		e := f.cfg.WOL[sel]
		if hw, err := parseMAC(e.MAC); err == nil {
			if err := sendWOL(hw, e.Broadcast, e.IP); err != nil {
				dialog.ShowError(err, f.w)
				return
			}
			dialog.ShowInformation("Wake-On-LAN", fmt.Sprintf(T("Signal sent to %d device(s)."), 1), f.w)
		}
	})
	del := widget.NewButtonWithIcon(T("Remove"), theme.DeleteIcon(), func() {
		if sel >= 0 && sel < len(f.cfg.WOL) {
			f.cfg.WOL = append(f.cfg.WOL[:sel], f.cfg.WOL[sel+1:]...)
			sel = -1
			f.cfg.save("")
			list.UnselectAll()
			list.Refresh()
		}
	})
	fromScan := widget.NewButton(T("Save MAC to WOL Manager"), func() {
		for _, d := range f.view {
			if d.MAC == nil {
				continue
			}
			m := formatMAC(d.MAC, 0)
			exists := false
			for _, w := range f.cfg.WOL {
				if strings.EqualFold(w.MAC, m) {
					exists = true
				}
			}
			if !exists {
				n := friendlyName(d, f.cfg)
				if n == "" {
					n = d.HostName
				}
				f.cfg.WOL = append(f.cfg.WOL, WOLEntry{Name: n, MAC: m, IP: d.IP.String()})
			}
		}
		f.cfg.save("")
		list.Refresh()
	})
	form := container.NewBorder(nil, nil, nil, add, container.NewGridWithColumns(2, name, mac))
	f.bigDialog(T("Wake-On-LAN Manager"), container.NewBorder(form, nil, nil, nil, list), wake, del, fromScan)
}

// lightTheme keeps the classic bright look of the Windows version.
type lightTheme struct{}

func (lightTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(n, theme.VariantLight)
}
func (lightTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (lightTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (lightTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	}
	return theme.DefaultTheme().Size(n)
}
