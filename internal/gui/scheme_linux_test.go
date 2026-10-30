package gui

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/testenv"
)

func TestRegisterScheme(t *testing.T) {
	for _, tc := range []struct {
		name     string
		flatpak  bool
		existing bool
	}{
		{"flatpak without entry", true, false},
		{"flatpak with entry", true, true},
		{"native without entry", false, false},
		{"native with entry", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FLATPAK_ID", "")
			if tc.flatpak {
				t.Setenv("FLATPAK_ID", "ai.usemagpie.Magpie")
			}
			root := t.TempDir()
			data := filepath.Join(root, "data")
			t.Setenv("XDG_DATA_HOME", data)
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(root, "calls")
			t.Setenv("MAGPIE_SCHEME_TEST_CALLS", calls)
			t.Setenv("PATH", bin)
			for _, name := range []string{"update-desktop-database", "xdg-mime"} {
				script := "#!/bin/sh\nprintf '%s\\n' '" + name + "' \"$@\" >> \"$MAGPIE_SCHEME_TEST_CALLS\"\n"
				testenv.Program(t, filepath.Join(bin, name), script)
			}
			dir := filepath.Join(data, "applications")
			path := filepath.Join(dir, "magpie.desktop")
			const old = "[Desktop Entry]\nExec=/usr/bin/magpie %u\nMimeType=x-scheme-handler/magpie;\n"
			if tc.existing {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := registerScheme(); err != nil {
				t.Fatal(err)
			}
			if tc.flatpak {
				if tc.existing {
					if got, err := os.ReadFile(path); err != nil || string(got) != old {
						t.Fatalf("existing desktop entry changed: %q, %v", got, err)
					}
				} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("applications directory was created: %v", err)
				}
				if _, err := os.Stat(filepath.Join(data, "icons")); !os.IsNotExist(err) {
					t.Fatalf("icon was installed: %v", err)
				}
				if got, err := os.ReadFile(calls); !os.IsNotExist(err) {
					t.Fatalf("desktop registration ran: %q, %v", got, err)
				}
				return
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(got), "Exec="+exe+" %u\n") || !strings.Contains(string(got), "MimeType=x-scheme-handler/magpie;\n") {
				t.Fatalf("desktop entry: %q, %v", got, err)
			}
			checkDesktopBinding(t, data, string(got))
			// An entry for this executable is already registered on the next run.
			if err := registerScheme(); err != nil {
				t.Fatal(err)
			}
			want := "update-desktop-database\n" + dir + "\nxdg-mime\ndefault\nmagpie.desktop\nx-scheme-handler/magpie\n"
			if got, err := os.ReadFile(calls); err != nil || string(got) != want {
				t.Fatalf("registration calls: %q, want %q (%v)", got, want, err)
			}
		})
	}
}

// checkDesktopBinding: the entry names the window's class and an icon that
// is installed, so neither the dock nor the app grid draws a gear (#2626).
func checkDesktopBinding(t *testing.T, data, entry string) {
	t.Helper()
	if !strings.Contains(entry, "\nStartupWMClass="+programName+"\n") || !strings.Contains(entry, "\nIcon=magpie\n") {
		t.Fatalf("desktop entry doesn't bind the window %q and its icon: %q", programName, entry)
	}
	icon, err := os.ReadFile(filepath.Join(data, "icons", "hicolor", "256x256", "apps", "magpie.png"))
	if err != nil || !bytes.Equal(icon, desktopIcon) {
		t.Fatalf("icon not installed: %d bytes, %v", len(icon), err)
	}
	if c, err := png.DecodeConfig(bytes.NewReader(icon)); err != nil || c.Width != 256 || c.Height != 256 {
		t.Fatalf("icon in 256x256 is %dx%d (%v)", c.Width, c.Height, err)
	}
}

// An entry an older magpie (or install.sh) wrote for this executable has no
// StartupWMClass; it is written again, once, and the icon put beside it.
func TestRegisterSchemeBindsAnOlderEntry(t *testing.T) {
	t.Setenv("FLATPAK_ID", "")
	root := t.TempDir()
	data := filepath.Join(root, "data")
	t.Setenv("XDG_DATA_HOME", data)
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(root, "calls")
	t.Setenv("MAGPIE_SCHEME_TEST_CALLS", calls)
	t.Setenv("PATH", bin)
	for _, name := range []string{"update-desktop-database", "xdg-mime"} {
		testenv.Program(t, filepath.Join(bin, name), "#!/bin/sh\nprintf '%s\\n' '"+name+"' >> \"$MAGPIE_SCHEME_TEST_CALLS\"\n")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(data, "applications")
	path := filepath.Join(dir, "magpie.desktop")
	older := "[Desktop Entry]\nType=Application\nName=magpie\nComment=Every agent's model. One place.\nExec=" + exe +
		" %u\nIcon=magpie\nCategories=Development;Utility;\nMimeType=x-scheme-handler/magpie;\nTerminal=false\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := registerScheme(); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkDesktopBinding(t, data, string(got))
	if c, err := os.ReadFile(calls); err != nil || string(c) != "update-desktop-database\nxdg-mime\n" {
		t.Fatalf("registered %q, want once (%v)", c, err)
	}
}

// The icon is there (install.sh or an earlier start put it) after another
// app built ~/.local/share/icons/hicolor/icon-theme.cache, which doesn't list
// it. GTK trusts that cache, so the app grid drew a gear until it is rebuilt
// or no longer trusted, with or without gtk-update-icon-cache installed.
func TestInstallIconUnderAStaleCache(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tool, cache  bool
		installedNow bool
	}{
		{"cache older than the icon, tool installed", true, true, false},
		{"cache older than the icon, no tool", false, true, false},
		{"cache older than a fresh install, tool installed", true, true, true},
		{"no cache", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			data := filepath.Join(root, "data")
			theme := filepath.Join(data, "icons", "hicolor")
			icon := filepath.Join(theme, "256x256", "apps", "magpie.png")
			cache := filepath.Join(theme, "icon-theme.cache")
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(root, "calls")
			t.Setenv("MAGPIE_SCHEME_TEST_CALLS", calls)
			t.Setenv("PATH", bin)
			if tc.tool { // lists every icon in the theme it is given, as the real one does
				testenv.Program(t, filepath.Join(bin, "gtk-update-icon-cache"), "#!/bin/sh\n"+
					"printf '%s\\n' \"$@\" >> \"$MAGPIE_SCHEME_TEST_CALLS\"\n"+
					"for a; do theme=$a; done\n"+
					"for f in \"$theme\"/*/apps/*.png; do n=${f##*/}; printf '%s\\0' \"${n%.png}\"; done > \"$theme/icon-theme.cache\"\n")
			}
			if err := os.MkdirAll(filepath.Dir(icon), 0o755); err != nil {
				t.Fatal(err)
			}
			// another app's cache, an hour old; the icon came half an hour later
			hour := time.Now().Add(-time.Hour)
			if !tc.installedNow {
				if err := os.WriteFile(icon, desktopIcon, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(icon, hour.Add(30*time.Minute), hour.Add(30*time.Minute)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.cache {
				if err := os.WriteFile(cache, []byte("someapp\x00"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(cache, hour, hour); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chtimes(theme, hour, hour); err != nil {
				t.Fatal(err)
			}
			for range 2 { // the second start finds nothing left to do
				if err := installIcon(data); err != nil {
					t.Fatal(err)
				}
			}
			if !gtkFindsIcon(t, theme, "magpie") {
				t.Fatal("GTK still reads a cache without magpie in it")
			}
			if !tc.cache {
				if _, err := os.Stat(cache); !os.IsNotExist(err) {
					t.Fatalf("a cache was made where there was none: %v", err)
				}
			}
			got, _ := os.ReadFile(calls)
			if n := strings.Count(string(got), theme+"\n"); n > 1 {
				t.Fatalf("cache rebuilt %d times, want at most once: %q", n, got)
			}
		})
	}
}

// gtkFindsIcon is GTK's own rule: a theme's icon-theme.cache is used while
// it is no older than the theme's folder, and then only for what it lists.
func gtkFindsIcon(t *testing.T, theme, name string) bool {
	t.Helper()
	c, err := os.Stat(filepath.Join(theme, "icon-theme.cache"))
	if os.IsNotExist(err) {
		return true
	}
	d, derr := os.Stat(theme)
	if err != nil || derr != nil {
		t.Fatal(err, derr)
	}
	if c.ModTime().Before(d.ModTime()) {
		return true
	}
	b, err := os.ReadFile(filepath.Join(theme, "icon-theme.cache"))
	return err == nil && bytes.Contains(b, []byte(name+"\x00"))
}

// WebKitGTK names its data folder after the program; a release run as
// downloaded kept the pages' localStorage under its executable's name. It
// moves to programName's folder, never over one that is there already.
func TestKeepWebData(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Base(exe)
	for _, tc := range []struct {
		name          string
		webkit, there bool
		moved         bool
	}{
		{"webkit data under the executable's name", true, false, true},
		{"magpie's folder there already", true, true, false},
		{"a folder that isn't WebKit's", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := t.TempDir()
			t.Setenv("XDG_DATA_HOME", data)
			from, to := filepath.Join(data, old), filepath.Join(data, programName)
			sub := "other"
			if tc.webkit {
				sub = "localstorage"
			}
			if err := os.MkdirAll(filepath.Join(from, sub), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(from, sub, "file__0.localstorage"), []byte("favourites"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.there { // even empty, it is magpie's, which a rename would replace
				if err := os.MkdirAll(to, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			keepWebData()
			b, err := os.ReadFile(filepath.Join(to, sub, "file__0.localstorage"))
			if moved := err == nil && string(b) == "favourites"; moved != tc.moved {
				t.Fatalf("moved to %s: %v, want %v (%v)", to, moved, tc.moved, err)
			}
			if _, err := os.Stat(from); os.IsNotExist(err) != tc.moved {
				t.Fatalf("%s left behind: %v", from, err)
			}
		})
	}
}
