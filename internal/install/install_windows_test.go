//go:build windows

package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubMenuEntry replaces the PowerShell shortcut call with a file write so the
// install flow is testable where PowerShell is unavailable (e.g. Wine).
func stubMenuEntry(t *testing.T) {
	orig := createShortcut
	createShortcut = func(lnk, target string) error { return os.WriteFile(lnk, []byte(target), 0o644) }
	t.Cleanup(func() { createShortcut = orig })
}

func TestUserPathsWindows(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\u\AppData\Local`)
	t.Setenv("APPDATA", `C:\Users\u\AppData\Roaming`)
	p, err := UserPaths()
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\Users\u\AppData\Local\Programs\PGP Tools\pgp-tools.exe`; p.Binary() != want {
		t.Errorf("Binary = %q, want %q", p.Binary(), want)
	}
	if want := `C:\Users\u\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\PGP Tools.lnk`; p.MenuEntry() != want {
		t.Errorf("MenuEntry = %q, want %q", p.MenuEntry(), want)
	}
}

func TestInPathCaseInsensitive(t *testing.T) {
	t.Setenv("PATH", `C:\Windows;C:\USERS\U\APPDATA\LOCAL\PROGRAMS\PGP TOOLS`)
	if !InPath(`C:\Users\u\AppData\Local\Programs\PGP Tools`) {
		t.Fatal("PATH match must ignore case on Windows")
	}
}

// TestRealShortcut exercises PowerShell + WScript.Shell on a real Windows host.
func TestRealShortcut(t *testing.T) {
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		t.Skip("powershell.exe not available")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "it's $odd & spaced", "pgp-tools.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(dir, "PGP Tools.lnk")
	if err := createShortcut(lnk, target); err != nil {
		t.Skipf("shortcut creation unavailable (restricted PowerShell?): %v", err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		`(New-Object -ComObject WScript.Shell).CreateShortcut($env:L).TargetPath`)
	cmd.Env = append(os.Environ(), "L="+lnk)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); !strings.EqualFold(got, target) {
		t.Fatalf("shortcut target = %q, want %q", got, target)
	}
}
