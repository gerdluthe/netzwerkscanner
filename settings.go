package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Bookmark struct {
	Name  string   `json:"name"`
	From  string   `json:"from"`
	To    string   `json:"to"`
	Extra []string `json:"extra,omitempty"`
}

type WOLEntry struct {
	Name      string `json:"name"`
	MAC       string `json:"mac"`
	IP        string `json:"ip"`
	Broadcast string `json:"broadcast"`
}

type Settings struct {
	// General
	MaxThreads      int  `json:"maxThreads"`
	PingTimeout     int  `json:"pingTimeout"`
	PingMethod      int  `json:"pingMethod"` // 0 ICMP, 1 ARP, 2 both
	Attempts        int  `json:"attempts"`
	AlwaysAnalyse   bool `json:"alwaysAnalyse"`
	ShowPingMethod  bool `json:"showPingMethod"`
	DisplayTTL      bool `json:"displayTTL"`
	DisplayInactive bool `json:"displayInactive"`
	ShowLastScan    bool `json:"showLastScan"`
	ScanRandom      bool `json:"scanRandom"`
	PingBroadcast   bool `json:"pingBroadcast"`
	DetectDupMAC    bool `json:"detectDupMac"`

	// Additional
	ResolveHostNames bool `json:"resolveHostNames"`
	HostNameMethod   int  `json:"hostNameMethod"` // 0 DNS+NetBIOS, 1 DNS only, 2 NetBIOS only
	ResolveMAC       bool `json:"resolveMac"`
	LookupVendor     bool `json:"lookupVendor"`
	GrabHTTP         bool `json:"grabHttp"`
	DeviceTypeGuess  bool `json:"deviceTypeGuess"`

	// Workstation
	LookupWorkgroup bool `json:"lookupWorkgroup"`
	LookupUser      bool `json:"lookupUser"`

	// Ports
	CheckPorts   bool   `json:"checkPorts"`
	Ports        string `json:"ports"`
	ServiceNames bool   `json:"serviceNames"`
	PortsAsNodes bool   `json:"portsAsNodes"`

	// Shares
	EnumShares        bool `json:"enumShares"`
	ShowAdminShares   bool `json:"showAdminShares"`
	ShowPrinterShares bool `json:"showPrinterShares"`

	// View
	GridLines     bool   `json:"gridLines"`
	AlternateRows bool   `json:"alternateRows"`
	MACFormat     int    `json:"macFormat"` // 0 AA-BB, 1 AA:BB, 2 aabb, 3 aa:bb
	CSVDelim      string `json:"csvDelim"`
	AutoExpand    bool   `json:"autoExpand"`
	HighlightNew  bool   `json:"highlightNew"`

	// State
	WindowW       int               `json:"windowW"`
	WindowH       int               `json:"windowH"`
	Maximized     bool              `json:"maximized"`
	FromIP        string            `json:"fromIP"`
	ToIP          string            `json:"toIP"`
	ExtraRanges   []string          `json:"extraRanges,omitempty"`
	Bookmarks     []Bookmark        `json:"bookmarks,omitempty"`
	FriendlyNames map[string]string `json:"friendlyNames,omitempty"`
	WOL           []WOLEntry        `json:"wol,omitempty"`
	ColumnWidths  map[string]int    `json:"columnWidths,omitempty"`
	HiddenColumns map[string]bool   `json:"hiddenColumns,omitempty"`
	KnownDevices  map[string]string `json:"knownDevices,omitempty"`
	Ignored       []string          `json:"ignored,omitempty"`
	LastFile      string            `json:"lastFile,omitempty"`
	Language      string            `json:"language"`
}

func defaultSettings() *Settings {
	return &Settings{
		MaxThreads: 128, PingTimeout: 1000, PingMethod: 2, Attempts: 1,
		ResolveHostNames: true, ResolveMAC: true, LookupVendor: true, DeviceTypeGuess: true,
		Ports: "21-23,80,443,445,3389,8080", ServiceNames: false,
		ShowAdminShares: true, ShowPrinterShares: true,
		MACFormat: 0, CSVDelim: ";", AutoExpand: true, HighlightNew: true,
		WindowW: 924, WindowH: 676, Language: "de",
		FromIP: "0.0.0.0", ToIP: "0.0.0.0",
		FriendlyNames: map[string]string{}, ColumnWidths: map[string]int{}, HiddenColumns: map[string]bool{},
		KnownDevices: map[string]string{},
	}
}

var settingsMu sync.Mutex

// configDirOverride is used on platforms (Android) where the app gets its own storage folder.
var configDirOverride string

func configDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "NetzScan")
}

func configPath() string { return filepath.Join(configDir(), "config.json") }

func loadSettings(path string) *Settings {
	s := defaultSettings()
	if path == "" {
		path = configPath()
	}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, s)
	}
	if s.FriendlyNames == nil {
		s.FriendlyNames = map[string]string{}
	}
	if s.ColumnWidths == nil {
		s.ColumnWidths = map[string]int{}
	}
	if s.HiddenColumns == nil {
		s.HiddenColumns = map[string]bool{}
	}
	if s.KnownDevices == nil {
		s.KnownDevices = map[string]string{}
	}
	if s.MaxThreads < 1 {
		s.MaxThreads = 1
	}
	if s.MaxThreads > 1024 {
		s.MaxThreads = 1024
	}
	if s.PingTimeout < 50 {
		s.PingTimeout = 50
	}
	if s.Attempts < 1 {
		s.Attempts = 1
	}
	return s
}

func (s *Settings) save(path string) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if path == "" {
		os.MkdirAll(configDir(), 0755)
		path = configPath()
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (s *Settings) clone() *Settings {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	b, _ := json.Marshal(s)
	c := &Settings{}
	json.Unmarshal(b, c)
	return c
}
