package main

import (
	"os"
	"runtime"
	"strings"
)

func argValue(args []string, name string) (string, bool) {
	for _, a := range args {
		l := strings.ToLower(a)
		for _, p := range []string{"/", "-", "--"} {
			if strings.HasPrefix(l, p+name+":") || strings.HasPrefix(l, p+name+"=") {
				return strings.Trim(a[len(p)+len(name)+1:], `"`), true
			}
			if l == p+name {
				return "", true
			}
		}
	}
	return "", false
}

func main() {
	runtime.LockOSThread()
	args := os.Args[1:]
	cfgPath, _ := argValue(args, "config")
	cfg := loadSettings(cfgPath)
	setLanguage(cfg.Language)

	rng, hasRange := argValue(args, "range")
	if hasRange && rng != "" {
		cfg.FromIP, cfg.ToIP, cfg.ExtraRanges = "0.0.0.0", "0.0.0.0", nil
		for _, r := range strings.Split(rng, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				cfg.ExtraRanges = append(cfg.ExtraRanges, r)
			}
		}
	}
	if out, ok := argValue(args, "auto"); ok && out != "" {
		os.Exit(runHeadless(cfg, out))
	}
	runGUI(cfg)
}

// runHeadless scans and writes the results to a file (command-line mode).
func runHeadless(cfg *Settings, out string) int {
	targets, err := buildTargets(cfg.FromIP, cfg.ToIP, cfg.ExtraRanges, cfg.Ignored, cfg.ScanRandom)
	if err != nil {
		os.WriteFile(out+".error.txt", []byte(err.Error()), 0644)
		return 2
	}
	sc := NewScanner(cfg, targets)
	sc.Start()
	var devs []*Device
	for d := range sc.Results {
		devs = append(devs, d)
	}
	sortDevicesByIP(devs)
	var cols []*ColumnDef
	for _, c := range allColumns {
		if c.Visible(cfg) {
			cols = append(cols, c)
		}
	}
	if err := saveResults(out, devs, cols, cfg, strings.Join(cfg.ExtraRanges, ", ")); err != nil {
		os.WriteFile(out+".error.txt", []byte(err.Error()), 0644)
		return 3
	}
	return 0
}
