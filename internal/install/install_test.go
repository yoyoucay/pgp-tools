package install

import (
	"os"
	"path/filepath"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	return Paths{
		BinDir:  filepath.Join(root, "my programs", "PGP Tools"),
		MenuDir: filepath.Join(root, "menu"),
		IconDir: filepath.Join(root, "icons"),
	}
}

var testAssets = Assets{DesktopEntry: []byte("Exec=@EXEC@\n"), IconSVG: []byte("<svg/>")}

func TestInstallUninstall(t *testing.T) {
	if !supported {
		t.Skip("install unsupported on this OS")
	}
	stubMenuEntry(t)
	p := testPaths(t)
	src := filepath.Join(t.TempDir(), "src-binary")
	if err := os.WriteFile(src, []byte("binary-v1"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := Install(p, src, testAssets); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(p.Binary()); err != nil || string(b) != "binary-v1" {
		t.Fatalf("binary not installed: %q %v", b, err)
	}
	for _, f := range p.menuFiles() {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("menu file missing: %v", err)
		}
	}

	// Re-installing from the installed copy must not corrupt it.
	if err := Install(p, p.Binary(), testAssets); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p.Binary()); string(b) != "binary-v1" {
		t.Fatal("self-install corrupted binary")
	}

	// Upgrading replaces the binary in place.
	if err := os.WriteFile(src, []byte("binary-v2"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Install(p, src, testAssets); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p.Binary()); string(b) != "binary-v2" {
		t.Fatal("upgrade did not replace binary")
	}

	if err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	for _, f := range append(p.menuFiles(), p.Binary()) {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("%s still present", f)
		}
	}
	if _, err := os.Stat(p.BinDir); !os.IsNotExist(err) {
		t.Fatal("empty install dir should be removed")
	}
	if err := Uninstall(p); err != nil {
		t.Fatalf("second uninstall should be a no-op: %v", err)
	}
}

func TestInstallMissingSource(t *testing.T) {
	if !supported {
		t.Skip("install unsupported on this OS")
	}
	stubMenuEntry(t)
	if err := Install(testPaths(t), filepath.Join(t.TempDir(), "nope"), testAssets); err == nil {
		t.Fatal("expected error for missing source binary")
	}
}

func TestUserPathsAbsolute(t *testing.T) {
	p, err := UserPaths()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	for _, d := range []string{p.BinDir, p.MenuDir} {
		if !filepath.IsAbs(d) {
			t.Errorf("path %q is not absolute", d)
		}
	}
}
