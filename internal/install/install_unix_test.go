//go:build !windows

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Linux writes plain files for the menu entry, so the real implementation is used.
func stubMenuEntry(*testing.T) {}

func TestDesktopEntryExec(t *testing.T) {
	if !supported {
		t.Skip("install unsupported on this OS")
	}
	p := testPaths(t)
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Install(p, src, testAssets); err != nil {
		t.Fatal(err)
	}
	entry, _ := os.ReadFile(p.MenuEntry())
	if want := `Exec="` + p.Binary() + `"`; strings.TrimSpace(string(entry)) != want {
		t.Fatalf("desktop entry = %q, want %q", entry, want)
	}
	if fi, _ := os.Stat(p.Binary()); fi.Mode().Perm() != 0o755 {
		t.Fatalf("binary mode = %v, want 0755", fi.Mode().Perm())
	}
}

func TestDesktopQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/home/u/.local/bin/pgp-tools": "/home/u/.local/bin/pgp-tools",
		"/home/a b/pgp-tools":          `"/home/a b/pgp-tools"`,
		"/opt/$x/pgp-tools":            `"/opt/\\$x/pgp-tools"`,
	} {
		if got := desktopQuote(in); got != want {
			t.Errorf("desktopQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUserPathsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	t.Setenv("XDG_BIN_HOME", "relative/ignored")
	p, err := UserPaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.MenuDir != "/xdg/data/applications" {
		t.Errorf("MenuDir = %q", p.MenuDir)
	}
	if strings.HasPrefix(p.BinDir, "relative") {
		t.Errorf("relative XDG_BIN_HOME must be ignored, got %q", p.BinDir)
	}
}
