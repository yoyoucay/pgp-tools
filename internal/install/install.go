// Package install performs a per-user desktop integration without admin/root rights:
// XDG desktop entry on Linux/BSD, Start Menu shortcut on Windows.
package install

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	appName     = "pgp-tools"
	displayName = "PGP Tools"
	description = "Encrypt and decrypt files with OpenPGP keys"
)

var (
	ErrUnsupportedOS = errors.New("install is not needed on this OS; run the app directly (it is portable)")
	// ErrMenuEntry means the binary was installed but the menu entry/shortcut was not.
	ErrMenuEntry = errors.New("app installed, but the menu shortcut could not be created")
)

// Paths are the per-user install targets.
type Paths struct {
	BinDir  string // binary location
	MenuDir string // Linux: applications dir; Windows: Start Menu Programs dir
	IconDir string // Linux only
}

// Assets are embedded files written next to the binary on install.
type Assets struct {
	DesktopEntry []byte // Linux .desktop template with @EXEC@ placeholder
	IconSVG      []byte
}

// Supported reports whether this OS uses a per-user install (Linux/BSD, Windows).
func Supported() bool { return supported }

func (p Paths) Binary() string { return filepath.Join(p.BinDir, appName+exeSuffix) }

// Install copies srcBinary into p.BinDir and adds the OS menu entry.
func Install(p Paths, srcBinary string, a Assets) error {
	if !supported {
		return ErrUnsupportedOS
	}
	for _, d := range p.dirs() {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	if err := copyBinary(srcBinary, p.Binary()); err != nil {
		return err
	}
	if err := installMenuEntry(p, a); err != nil {
		return fmt.Errorf("%w: %v", ErrMenuEntry, err)
	}
	return nil
}

// Uninstall removes installed files; missing files are not an error.
func Uninstall(p Paths) error {
	if !supported {
		return ErrUnsupportedOS
	}
	var errs []error
	for _, f := range p.menuFiles() {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	if err := removeBinary(p.Binary()); err != nil {
		errs = append(errs, err)
	}
	removeIfEmpty(p.BinDir)
	return errors.Join(errs...)
}

// InPath reports whether dir is listed in $PATH.
func InPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p != "" && samePathString(filepath.Clean(p), filepath.Clean(dir)) {
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
	// Installing from the already-installed copy: nothing to copy.
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
	return replaceFile(tmp.Name(), path)
}

// replaceFile renames src over dst. Windows refuses to overwrite a running .exe but
// allows renaming it, so on failure the old file is moved aside first.
func replaceFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		return fmt.Errorf("install %s: %w", dst, err)
	}
	aside := fmt.Sprintf("%s.old-%d", dst, time.Now().UnixNano())
	if mvErr := os.Rename(dst, aside); mvErr != nil {
		return fmt.Errorf("install %s: %w (file in use? close running copies and retry)", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(aside, dst)
		return fmt.Errorf("install %s: %w", dst, err)
	}
	_ = os.Remove(aside) // fails while the old copy still runs; harmless leftover
	return nil
}

// removeBinary deletes path. A running .exe on Windows cannot be deleted, so it is
// moved to the temp directory, where the OS cleanup removes it later.
func removeBinary(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	aside := filepath.Join(os.TempDir(), fmt.Sprintf("%s-removed-%d%s", appName, time.Now().UnixNano(), exeSuffix))
	if mvErr := os.Rename(path, aside); mvErr != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}
