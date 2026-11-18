//go:build !windows

package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const exeSuffix = ""

// macOS apps are dragged to /Applications; no per-user menu integration is needed.
var supported = runtime.GOOS != "darwin"

func (p Paths) MenuEntry() string { return filepath.Join(p.MenuDir, appName+".desktop") }
func (p Paths) Icon() string      { return filepath.Join(p.IconDir, appName+".svg") }

func (p Paths) dirs() []string      { return []string{p.BinDir, p.MenuDir, p.IconDir} }
func (p Paths) menuFiles() []string { return []string{p.MenuEntry(), p.Icon()} }

// UserPaths follows the XDG Base Directory spec; falls back to ~/.local.
func UserPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	data := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return Paths{
		BinDir:  envOr("XDG_BIN_HOME", filepath.Join(home, ".local", "bin")),
		MenuDir: filepath.Join(data, "applications"),
		IconDir: filepath.Join(data, "icons", "hicolor", "scalable", "apps"),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" && filepath.IsAbs(v) {
		return v
	}
	return fallback
}

func installMenuEntry(p Paths, a Assets) error {
	if err := writeAtomic(p.Icon(), a.IconSVG, 0o644); err != nil {
		return err
	}
	entry := bytes.ReplaceAll(a.DesktopEntry, []byte("@EXEC@"), []byte(desktopQuote(p.Binary())))
	return writeAtomic(p.MenuEntry(), entry, 0o644)
}

func samePathString(a, b string) bool { return a == b }

// desktopQuote quotes an Exec path per the Desktop Entry spec when it has reserved chars.
func desktopQuote(path string) string {
	if !strings.ContainsAny(path, " \t\"'\\$`") {
		return path
	}
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", `$`, `\\$`)
	return `"` + r.Replace(path) + `"`
}
