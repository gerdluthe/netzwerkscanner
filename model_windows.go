package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"netzscan/third_party/walk"
)

// ---------- table model ----------

type rowKind int

const (
	rowDevice rowKind = iota
	rowShare
	rowPort
)

type row struct {
	d    *Device
	kind rowKind
	idx  int
}

type Filter struct {
	Col   int
	Op    string
	Value string
}

type DevModel struct {
	walk.TableModelBase
	walk.SorterBase
	cfg     *Settings
	devices []*Device
	byIP    map[string]*Device
	rows    []row
	visible []*Device

	sortCol   int
	sortOrder walk.SortOrder
	sorted    bool

	filters    []Filter
	onlyShares bool
}

func NewDevModel(cfg *Settings) *DevModel {
	return &DevModel{cfg: cfg, byIP: map[string]*Device{}, sortCol: 0, sortOrder: walk.SortAscending}
}

func (m *DevModel) RowCount() int { return len(m.rows) }

func (m *DevModel) Value(r, col int) interface{} {
	if r < 0 || r >= len(m.rows) || col < 0 || col >= len(allColumns) {
		return ""
	}
	rw := m.rows[r]
	switch rw.kind {
	case rowShare:
		s := rw.d.Shares[rw.idx]
		switch allColumns[col].ID {
		case "ip":
			return s.Name
		case "mac":
			return s.Kind()
		case "rtt":
			return ""
		case "host":
			return s.Remark
		}
		return ""
	case rowPort:
		p := rw.d.Ports[rw.idx]
		switch allColumns[col].ID {
		case "ip":
			return fmt.Sprintf(T("Port %d"), p)
		case "mac":
			return serviceName(p)
		}
		return ""
	}
	return allColumns[col].Get(rw.d, m.cfg)
}

func (m *DevModel) RowIndent(r int) int {
	if r >= 0 && r < len(m.rows) && m.rows[r].kind != rowDevice {
		return 1
	}
	return 0
}

func (m *DevModel) ID(r int) interface{} {
	if r < 0 || r >= len(m.rows) {
		return nil
	}
	rw := m.rows[r]
	return fmt.Sprintf("%s|%d|%d", rw.d.IP, rw.kind, rw.idx)
}

func (m *DevModel) Row(r int) (row, bool) {
	if r < 0 || r >= len(m.rows) {
		return row{}, false
	}
	return m.rows[r], true
}

func (m *DevModel) ColumnSortable(col int) bool { return true }

func (m *DevModel) Sort(col int, order walk.SortOrder) error {
	m.sortCol, m.sortOrder, m.sorted = col, order, true
	m.rebuild()
	m.PublishRowsReset()
	return m.SorterBase.Sort(col, order)
}

func (m *DevModel) RemoveSorting() {
	m.sortCol, m.sortOrder = 0, walk.SortAscending
	m.rebuild()
	m.PublishRowsReset()
}

func (m *DevModel) hasChildren(d *Device) bool {
	return len(d.Shares) > 0 || (m.cfg.PortsAsNodes && len(d.Ports) > 0)
}

func (m *DevModel) matches(d *Device) bool {
	if m.onlyShares && len(d.Shares) == 0 {
		return false
	}
	for _, f := range m.filters {
		if f.Col < 0 || f.Col >= len(allColumns) {
			continue
		}
		v := strings.ToLower(allColumns[f.Col].Get(d, m.cfg))
		t := strings.ToLower(f.Value)
		ok := true
		switch f.Op {
		case "contains":
			ok = strings.Contains(v, t)
		case "does not contain":
			ok = !strings.Contains(v, t)
		case "equals":
			ok = v == t
		case "not equal":
			ok = v != t
		case "starts with":
			ok = strings.HasPrefix(v, t)
		case "ends with":
			ok = strings.HasSuffix(v, t)
		case "is empty":
			ok = v == ""
		case "is not empty":
			ok = v != ""
		case "greater than", "less than":
			a, e1 := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, " ms")), 64)
			b, e2 := strconv.ParseFloat(t, 64)
			if e1 != nil || e2 != nil {
				ok = false
			} else if f.Op == "greater than" {
				ok = a > b
			} else {
				ok = a < b
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (m *DevModel) rebuild() {
	vis := m.visible[:0]
	for _, d := range m.devices {
		if m.matches(d) {
			vis = append(vis, d)
		}
	}
	m.visible = vis
	col := allColumns[m.sortCol]
	asc := m.sortOrder == walk.SortAscending
	sort.SliceStable(m.visible, func(i, j int) bool {
		a, b := m.visible[i], m.visible[j]
		var less, eq bool
		if col.Num != nil {
			x, y := col.Num(a), col.Num(b)
			less, eq = x < y, x == y
		} else {
			x, y := strings.ToLower(col.Get(a, m.cfg)), strings.ToLower(col.Get(b, m.cfg))
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
		if asc {
			return less
		}
		return !less
	})
	m.rows = m.rows[:0]
	for _, d := range m.visible {
		m.rows = append(m.rows, row{d: d})
		if d.Expanded {
			for i := range d.Shares {
				m.rows = append(m.rows, row{d: d, kind: rowShare, idx: i})
			}
			if m.cfg.PortsAsNodes {
				for i := range d.Ports {
					m.rows = append(m.rows, row{d: d, kind: rowPort, idx: i})
				}
			}
		}
	}
}

func (m *DevModel) Refresh() {
	m.rebuild()
	m.PublishRowsReset()
}

// Upsert adds or replaces a device, keeping UI state.
func (m *DevModel) Upsert(d *Device) {
	if old, ok := m.byIP[d.Key()]; ok {
		d.Expanded = old.Expanded
		d.Highlighted = old.Highlighted
		*old = *d
		return
	}
	m.byIP[d.Key()] = d
	m.devices = append(m.devices, d)
}

func (m *DevModel) Remove(d *Device) {
	delete(m.byIP, d.Key())
	for i, x := range m.devices {
		if x == d {
			m.devices = append(m.devices[:i], m.devices[i+1:]...)
			break
		}
	}
}

func (m *DevModel) Clear() {
	m.devices = nil
	m.byIP = map[string]*Device{}
	m.Refresh()
}

func (m *DevModel) Counts() (online, total, shown int) {
	for _, d := range m.devices {
		if d.Alive {
			online++
		}
	}
	return online, len(m.devices), len(m.visible)
}

func (m *DevModel) RowOf(d *Device) int {
	for i, r := range m.rows {
		if r.d == d && r.kind == rowDevice {
			return i
		}
	}
	return -1
}
