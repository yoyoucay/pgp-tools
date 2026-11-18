//go:build windows

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const exeSuffix = ".exe"

var supported = true

func (p Paths) MenuEntry() string { return filepath.Join(p.MenuDir, displayName+".lnk") }

func (p Paths) dirs() []string      { return []string{p.BinDir, p.MenuDir} }
func (p Paths) menuFiles() []string { return []string{p.MenuEntry()} }

// UserPaths uses per-user locations that never need administrator rights:
// %LOCALAPPDATA%\Programs\PGP Tools and the user's Start Menu.
func UserPaths() (Paths, error) {
	local, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return Paths{}, fmt.Errorf("resolve LOCALAPPDATA: %w", err)
	}
	roaming, err := os.UserConfigDir() // %APPDATA%
	if err != nil {
		return Paths{}, fmt.Errorf("resolve APPDATA: %w", err)
	}
	return Paths{
		BinDir:  filepath.Join(local, "Programs", displayName),
		MenuDir: filepath.Join(roaming, "Microsoft", "Windows", "Start Menu", "Programs"),
	}, nil
}

func installMenuEntry(p Paths, _ Assets) error {
	return createShortcut(p.MenuEntry(), p.Binary())
}

func samePathString(a, b string) bool { return strings.EqualFold(a, b) }

// createShortcut writes a .lnk via the WScript.Shell COM object. Paths travel in
// environment variables, never in the script text, so they need no escaping.
// A variable so tests can substitute it.
var createShortcut = func(lnk, target string) error {
	const script = `$ErrorActionPreference='Stop';` +
		`$s=(New-Object -ComObject WScript.Shell).CreateShortcut($env:PGPTOOLS_LNK);` +
		`$s.TargetPath=$env:PGPTOOLS_TARGET;` +
		`$s.WorkingDirectory=$env:PGPTOOLS_WORKDIR;` +
		`$s.Description=$env:PGPTOOLS_DESC;` +
		`$s.Save()`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(),
		"PGPTOOLS_LNK="+lnk,
		"PGPTOOLS_TARGET="+target,
		"PGPTOOLS_WORKDIR="+filepath.Dir(target),
		"PGPTOOLS_DESC="+description,
	)
	// CREATE_NO_WINDOW: no console flash when called from the GUI.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Restricted or stub PowerShell can exit 0 without writing the file.
	if _, err := os.Stat(lnk); err != nil {
		return fmt.Errorf("powershell reported success but no shortcut was written: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
