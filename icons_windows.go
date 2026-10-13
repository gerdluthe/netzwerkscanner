package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"sync"

	"github.com/lxn/win"
	xdraw "golang.org/x/image/draw"

	"netzscan/third_party/walk"
)

//go:embed assets/icons/*.png
var iconFS embed.FS

var pngSizes = []int{16, 20, 24, 32, 48, 64}

type iconKey struct {
	name string
	px   int
}

var (
	iconMu    sync.Mutex
	iconCache = map[iconKey]*walk.Bitmap{}
	// toolbar bitmap -> menu-size bitmap
	menuMap = map[walk.Image]*walk.Bitmap{}
)

func loadPNG(name string, px int) image.Image {
	// choose smallest available size >= px
	var best []byte
	for _, s := range pngSizes {
		b, err := iconFS.ReadFile(fmt.Sprintf("assets/icons/%s_%d.png", name, s))
		if err != nil {
			continue
		}
		best = b
		if s >= px {
			break
		}
	}
	if best == nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(best))
	if err != nil {
		return nil
	}
	if img.Bounds().Dx() == px {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, px, px))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// bitmapPx returns a bitmap of exactly px pixels (device pixels).
func bitmapPx(name string, px int, dpi int) *walk.Bitmap {
	iconMu.Lock()
	defer iconMu.Unlock()
	k := iconKey{name, px}
	if b, ok := iconCache[k]; ok {
		return b
	}
	img := loadPNG(name, px)
	if img == nil {
		return nil
	}
	b, err := walk.NewBitmapFromImageForDPI(img, dpi)
	if err != nil {
		return nil
	}
	iconCache[k] = b
	return b
}

// smallIcon returns an icon sized like the system small icon (list views, menus).
func smallIcon(name string, dpi int) *walk.Bitmap {
	px := int(win.GetSystemMetricsForDpi(win.SM_CXSMICON, uint32(dpi)))
	if px <= 0 {
		px = 16 * dpi / 96
	}
	return bitmapPx(name, px, dpi)
}

// toolIcon returns a 24px (at 96 DPI) toolbar icon and registers a matching menu icon.
func toolIcon(name string, dpi int) *walk.Bitmap {
	b := bitmapPx(name, 24*dpi/96, dpi)
	if b != nil {
		m := smallIcon(name, dpi)
		iconMu.Lock()
		menuMap[b] = m
		iconMu.Unlock()
	}
	return b
}

func init() {
	walk.MenuImageFor = func(img walk.Image, dpi int) *walk.Bitmap {
		iconMu.Lock()
		defer iconMu.Unlock()
		return menuMap[img]
	}
}

var btnIcons = map[iconKey]*walk.Icon{}

// btnIcon returns a 16px (at 96 DPI) icon for push buttons (alpha-safe).
func btnIcon(name string, dpi int) *walk.Icon {
	px := 16 * dpi / 96
	iconMu.Lock()
	defer iconMu.Unlock()
	k := iconKey{name, px}
	if ic, ok := btnIcons[k]; ok {
		return ic
	}
	img := loadPNG(name, px)
	if img == nil {
		return nil
	}
	ic, err := walk.NewIconFromImageForDPI(img, dpi)
	if err != nil {
		return nil
	}
	btnIcons[k] = ic
	return ic
}
