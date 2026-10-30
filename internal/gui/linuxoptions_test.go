//go:build !nogui

package gui

import (
	"bytes"
	"image/png"
	"runtime"
	"testing"
)

// GTK 3 on X11 hands the window manager no icon at all for one of 512px or
// more, so the window had no _NET_WM_ICON (#2626).
func TestLinuxWindowIconFitsGTK3(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("GTK 3 is Linux's")
	}
	c, err := png.DecodeConfig(bytes.NewReader(appIconFor()))
	if err != nil || c.Width >= 512 || c.Height >= 512 {
		t.Fatalf("window icon is %dx%d (%v), want under 512px", c.Width, c.Height, err)
	}
}

// GTK makes the program name the window's Wayland app_id and X11 WM_CLASS.
// Left unset, both were the executable's name (magpie-linux-amd64 run as
// downloaded), which no desktop entry names, and the dock drew a gear (#2626).
func TestLinuxWindowNamedAsItsEntry(t *testing.T) {
	o := linuxOptions()
	if o.ProgramName != programName || programName != "magpie" {
		t.Fatalf("ProgramName = %q (programName %q), want the entry's StartupWMClass, magpie", o.ProgramName, programName)
	}
	if !o.DisableQuitOnLastWindowClosed {
		t.Fatal("magpie quits when its last window closes; the tray should stay")
	}
}
