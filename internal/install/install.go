// Package install performs a per-user (non-root) desktop integration on Linux/BSD.
package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const appName = "pgp-tools"

var ErrUnsupportedOS = errors.New("desktop install is only needed on Linux/BSD; on this OS run the binary directly (portable)")

// Paths are the per-user install targets, resolved from XDG variables.
type Paths struct {
	BinDir     string
	DesktopDir string
	IconDir    string
}

func (p Paths) Binary() string  { return filepath.Join(p.BinDir, appName) }
func (p Paths) Desktop() string { return filepath.Join(p.DesktopDir, appName+".desktop") }
func (p Paths) Icon() string    { return filepath.Join(p.IconDir, appName+".svg") }

// UserPaths follows the XDG Base Directory spec; falls back to ~/.local.
func UserPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	data := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return Paths{
		BinDir:     envOr("XDG_BIN_HOME", filepath.Join(home, ".local", "bin")),
		DesktopDir: filepath.Join(data, "applications"),
		IconDir:    filepath.Join(data, "icons", "hicolor", "scalable", "apps"),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" && filepath.IsAbs(v) {
		return v
	}
	return fallback
}

func supported() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		return true
	}
	return false
}

// Install copies srcBinary into p.BinDir and writes the desktop entry and icon.
func Install(p Paths, srcBinary string, desktopTmpl, icon []byte) error {
	if !supported() {
		return ErrUnsupportedOS
	}
	for _, d := range []string{p.BinDir, p.DesktopDir, p.IconDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	if err := copyBinary(srcBinary, p.Binary()); err != nil {
		return err
	}
	if err := writeAtomic(p.Icon(), icon, 0o644); err != nil {
		return err
	}
	entry := bytes.ReplaceAll(desktopTmpl, []byte("@EXEC@"), []byte(desktopQuote(p.Binary())))
	return writeAtomic(p.Desktop(), entry, 0o644)
}

// Uninstall removes installed files; missing files are not an error.
func Uninstall(p Paths) error {
	if !supported() {
		return ErrUnsupportedOS
	}
	var errs []error
	for _, f := range []string{p.Desktop(), p.Icon(), p.Binary()} {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// InPath reports whether dir is listed in $PATH.
func InPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

func copyBinary(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source binary: %w", err)
	}
	// Installing from the already-installed copy would truncate the running file.
	if dstInfo, err := os.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source binary: %w", err)
	}
	defer in.Close()
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read source binary: %w", err)
	}
	return writeAtomic(dst, data, 0o755)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}

// desktopQuote quotes an Exec path per the Desktop Entry spec when it has reserved chars.
func desktopQuote(path string) string {
	if !strings.ContainsAny(path, " \t\"'\\$`") {
		return path
	}
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", `$`, `\\$`)
	return `"` + r.Replace(path) + `"`
}
