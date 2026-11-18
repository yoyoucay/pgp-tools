package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallUninstall(t *testing.T) {
	if !supported() {
		t.Skip("desktop install unsupported on " + runtime.GOOS)
	}
	root := t.TempDir()
	p := Paths{
		BinDir:     filepath.Join(root, "my bin"),
		DesktopDir: filepath.Join(root, "apps"),
		IconDir:    filepath.Join(root, "icons"),
	}
	src := filepath.Join(root, "src-binary")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := Install(p, src, []byte("Exec=@EXEC@\n"), []byte("<svg/>")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p.Binary())
	if err != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("binary not installed executable: %v %v", fi, err)
	}
	entry, _ := os.ReadFile(p.Desktop())
	if want := `Exec="` + p.Binary() + `"`; strings.TrimSpace(string(entry)) != want {
		t.Fatalf("desktop entry = %q, want %q", entry, want)
	}

	// Re-installing from the installed copy must not corrupt it.
	if err := Install(p, p.Binary(), []byte("Exec=@EXEC@\n"), []byte("<svg/>")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p.Binary()); string(b) != "#!/bin/sh\n" {
		t.Fatal("self-install corrupted binary")
	}

	if err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{p.Binary(), p.Desktop(), p.Icon()} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("%s still present", f)
		}
	}
	if err := Uninstall(p); err != nil {
		t.Fatalf("second uninstall should be a no-op: %v", err)
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
	if p.DesktopDir != "/xdg/data/applications" {
		t.Errorf("DesktopDir = %q", p.DesktopDir)
	}
	if strings.HasPrefix(p.BinDir, "relative") {
		t.Errorf("relative XDG_BIN_HOME must be ignored, got %q", p.BinDir)
	}
}
