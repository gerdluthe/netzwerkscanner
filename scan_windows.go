package main

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"netzscan/third_party/walk"
)

func windowsLogicalDrives() (uint32, error) { return windows.GetLogicalDrives() }

func unsafeSizeofWP() int { return 44 }

func (a *App) rangeDescription() string {
	from, to := a.ipFrom.IPString(), a.ipTo.IPString()
	s := ""
	if from != "0.0.0.0" {
		s = from + " - " + to
	}
	if n := len(a.cfg.ExtraRanges); n > 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("(+%d)", n)
	}
	return s
}

func (a *App) toggleScan() {
	if a.scanner != nil {
		a.sbStatus.SetText(T("Stopping..."))
		a.scanner.Stop()
		return
	}
	a.startScan(nil)
}

// startScan scans either the range from the input bar or the given list.
func (a *App) startScan(list []net.IP) {
	if a.scanner != nil {
		return
	}
	cfg := a.cfg.clone()
	targets := list
	if targets == nil {
		var err error
		targets, err = buildTargets(a.ipFrom.IPString(), a.ipTo.IPString(), a.cfg.ExtraRanges, a.cfg.Ignored, cfg.ScanRandom)
		if err != nil {
			walk.MsgBox(a.mw, appName, err.Error(), walk.MsgBoxIconError)
			return
		}
		if len(targets) > 65536 {
			if walk.MsgBox(a.mw, appName, fmt.Sprintf(T("You are going to scan %d addresses. This may take a long time. Continue?"), len(targets)),
				walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
				return
			}
		}
		a.lastRange = a.rangeDescription()
		a.cfg.FromIP, a.cfg.ToIP = a.ipFrom.IPString(), a.ipTo.IPString()
		// a new range scan replaces the results (like the original)
		a.model.Clear()
	}
	a.applyColumnVisibility()

	a.firstEverScan = len(a.cfg.KnownDevices) == 0
	a.scanner = NewScanner(cfg, targets)
	a.scanner.Start()
	a.btnScan.SetText(T("Stop Scanning"))
	a.btnScan.SetImage(btnIcon("stop", a.dpi))
	a.ac["scan"].SetText(T("Stop Scanning"))
	a.btnPause.SetEnabled(true)
	a.progress.SetValue(0)
	for _, id := range []string{"clear", "load", "options", "rescanall", "rescanoffline"} {
		a.ac[id].SetEnabled(false)
	}
	sc := a.scanner
	a.scanDone = make(chan struct{})
	done := a.scanDone
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				a.mw.Synchronize(func() { a.flush(sc, false) })
			case <-done:
				return
			}
		}
	}()
}

func (a *App) flush(sc *Scanner, final bool) {
	if sc != a.scanner {
		return
	}
	changed := false
	closed := false
	for i := 0; i < 5000; i++ {
		select {
		case d, ok := <-sc.Results:
			if !ok {
				closed = true
				i = 1 << 30
				break
			}
			a.addResult(d)
			changed = true
		default:
			i = 1 << 30
		}
	}
	if changed {
		a.refresh()
	}
	a.updateStatus()
	if closed {
		a.finishScan(sc)
	}
}

func (a *App) addResult(d *Device) {
	key := d.IP.String()
	if d.MAC != nil {
		key = formatMAC(d.MAC, 0)
	}
	if d.Alive {
		if _, known := a.cfg.KnownDevices[key]; !known && !a.firstEverScan {
			d.New = true
		}
		if _, known := a.cfg.KnownDevices[key]; !known {
			a.cfg.KnownDevices[key] = time.Now().Format("2006-01-02 15:04")
		}
	}
	d.Expanded = a.cfg.AutoExpand
	if old, ok := a.model.byIP[d.Key()]; ok && old.New {
		d.New = true
	}
	a.model.Upsert(d)
}

func (a *App) finishScan(sc *Scanner) {
	if a.scanDone != nil {
		close(a.scanDone)
		a.scanDone = nil
	}
	elapsed := time.Since(sc.Started)
	a.scanner = nil
	a.btnScan.SetText(T("Start Scanning"))
	a.btnScan.SetImage(btnIcon("play", a.dpi))
	a.btnPause.SetEnabled(false)
	a.btnPause.SetImage(btnIcon("pause", a.dpi))
	a.ac["scan"].SetText(T("Start Scanning"))
	for _, id := range []string{"clear", "load", "options", "rescanall", "rescanoffline"} {
		a.ac[id].SetEnabled(true)
	}
	if a.cfg.DetectDupMAC {
		by := map[string]int{}
		for _, d := range a.model.devices {
			if d.MAC != nil {
				by[formatMAC(d.MAC, 0)]++
			}
		}
		for _, d := range a.model.devices {
			d.DupMAC = d.MAC != nil && by[formatMAC(d.MAC, 0)] > 1
		}
	}
	a.refresh()
	a.progress.SetValue(1000)
	a.updateStatus()
	a.sbTime.SetText(fmt.Sprintf(T("Scan time: %s"), elapsed.Round(time.Millisecond*100)))
	a.cfg.save("")
	win.MessageBeep(win.MB_OK)
}

func (a *App) togglePause() {
	if a.scanner == nil {
		return
	}
	p := !a.scanner.Paused()
	a.scanner.SetPaused(p)
	if p {
		a.btnPause.SetImage(btnIcon("pause_on", a.dpi))
	} else {
		a.btnPause.SetImage(btnIcon("pause", a.dpi))
	}
	a.updateStatus()
}

func (a *App) updateStatus() {
	online, total, shown := a.model.Counts()
	if shown != total {
		a.sbDevices.SetText(fmt.Sprintf(T("Devices: %d / %d (filtered: %d)"), online, total, shown))
	} else {
		a.sbDevices.SetText(fmt.Sprintf(T("Devices: %d / %d"), online, total))
	}
	sc := a.scanner
	if sc == nil {
		if a.rescanning > 0 {
			a.sbStatus.SetText(fmt.Sprintf(T("Rescanning %d..."), a.rescanning))
		} else {
			a.sbStatus.SetText(T("Ready"))
		}
		a.sbThreads.SetText(T("Threads: 0"))
		return
	}
	done, totalT := atomic.LoadInt64(&sc.Done), sc.Total
	pct := 0
	if totalT > 0 {
		pct = int(done * 1000 / totalT)
	}
	a.progress.SetValue(pct)
	if sc.Paused() {
		a.sbStatus.SetText(fmt.Sprintf(T("Paused (%d%%)"), pct/10))
	} else if sc.ctx.Err() != nil {
		a.sbStatus.SetText(T("Stopping..."))
	} else {
		a.sbStatus.SetText(fmt.Sprintf(T("Scanning... %d%%"), pct/10))
	}
	a.sbThreads.SetText(fmt.Sprintf(T("Threads: %d"), atomic.LoadInt64(&sc.Active)))
	a.sbTime.SetText(fmt.Sprintf(T("%d of %d  ·  %s"), done, totalT, time.Since(sc.Started).Round(time.Second)))
}

// rescanSelected rescans the selected devices in the background without clearing the list.
func (a *App) rescanSelected() {
	a.rescanDevices(a.selectedDevices())
}

func (a *App) rescanList(onlyOffline bool) {
	var ds []*Device
	for _, d := range a.model.visible {
		if !onlyOffline || !d.Alive {
			ds = append(ds, d)
		}
	}
	if len(ds) == 0 {
		return
	}
	if !onlyOffline {
		a.rescanDevices(ds)
		return
	}
	var ips []net.IP
	for _, d := range ds {
		ips = append(ips, d.IP)
	}
	a.startScan(ips)
}

func (a *App) rescanDevices(ds []*Device) {
	if len(ds) == 0 {
		return
	}
	if len(ds) > 8 && a.scanner == nil {
		var ips []net.IP
		for _, d := range ds {
			ips = append(ips, d.IP)
		}
		a.startScan(ips)
		return
	}
	cfg := a.cfg.clone()
	ports := []int(nil)
	if cfg.CheckPorts {
		ports = parsePorts(cfg.Ports)
	}
	nets := localNetworks()
	for _, d := range ds {
		ip := d.IP
		a.rescanning++
		go func() {
			nd := ScanSingle(context.Background(), cfg, ip, ports, nets)
			a.mw.Synchronize(func() {
				a.rescanning--
				if nd == nil {
					nd = &Device{IP: ip, RTT: -1, LastScan: time.Now()}
				}
				a.addResult(nd)
				a.refresh()
				a.updateStatus()
			})
		}()
	}
	a.updateStatus()
}
