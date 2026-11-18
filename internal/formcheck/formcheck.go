// Package formcheck turns raw form input into per-field, plain-language feedback.
// It has no UI dependency so every rule is unit-testable on any OS.
package formcheck

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

type Level int

const (
	Neutral Level = iota // hint for an untouched or optional field
	OK
	Warn  // usable, but the user should know something
	Error // blocks running
)

type Result struct {
	Level Level
	Text  string
}

func neutral(format string, a ...any) Result { return Result{Neutral, fmt.Sprintf(format, a...)} }
func ok(format string, a ...any) Result      { return Result{OK, fmt.Sprintf(format, a...)} }
func warn(format string, a ...any) Result    { return Result{Warn, fmt.Sprintf(format, a...)} }
func fail(format string, a ...any) Result    { return Result{Error, fmt.Sprintf(format, a...)} }

// Checker validates forms. It caches passphrase checks because unlocking a key
// runs a deliberately slow key-derivation function.
type Checker struct {
	mu    sync.Mutex
	cache map[[32]byte]error
}

func NewChecker() *Checker { return &Checker{cache: map[[32]byte]error{}} }

const maxCacheEntries = 64

func (c *Checker) checkPassphrase(keyPath, pass string) error {
	fi, err := os.Stat(keyPath)
	if err != nil {
		return err
	}
	// Cache key is a digest of file identity and passphrase; the passphrase itself is not kept.
	k := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%d\x00%s", keyPath, fi.Size(), fi.ModTime().UnixNano(), pass)))
	c.mu.Lock()
	res, hit := c.cache[k]
	c.mu.Unlock()
	if hit {
		return res
	}
	res = pgp.CheckPassphrase(keyPath, []byte(pass))
	c.mu.Lock()
	if len(c.cache) >= maxCacheEntries {
		c.cache = map[[32]byte]error{}
	}
	c.cache[k] = res
	c.mu.Unlock()
	return res
}

// NormalizePath cleans user-typed or pasted paths: surrounding whitespace and quotes
// (Windows "Copy as path" adds quotes), forward slashes on Windows, and a leading ~.
func NormalizePath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if s == "" {
		return ""
	}
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[1:])
		}
	}
	return filepath.Clean(filepath.FromSlash(s))
}

func checkReadableFile(path, emptyHint string) (Result, os.FileInfo) {
	if path == "" {
		return neutral("%s", emptyHint), nil
	}
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fail("File not found."), nil
	case errors.Is(err, fs.ErrPermission):
		return fail("Cannot open this file: access denied."), nil
	case err != nil:
		return fail("Cannot open this file: %v.", err), nil
	case fi.IsDir():
		return fail("This is a folder. Choose a file."), nil
	case !fi.Mode().IsRegular():
		return fail("This is not a regular file."), nil
	}
	return Result{}, fi
}

// inspectKey returns a non-zero Result when the key file is unusable.
func inspectKey(path, emptyHint string) (Result, *pgp.KeyInfo) {
	if r, fi := checkReadableFile(path, emptyHint); fi == nil {
		return r, nil
	}
	info, err := pgp.InspectKey(path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fail("Cannot open this file: access denied."), nil
		}
		return fail("Not an OpenPGP key file. Expected a .asc, .gpg or .key file exported from your PGP app."), nil
	}
	return Result{}, &info
}

func describeKey(k *pgp.KeyInfo) string {
	s := "Key ID " + k.KeyID
	if k.UserID != "" {
		s = k.UserID + ", key ID " + k.KeyID
	}
	if k.Count > 1 {
		s += fmt.Sprintf(" (+%d more keys)", k.Count-1)
	}
	return s
}

// passphraseResult decides the passphrase field state for a chosen private key.
func (c *Checker) passphraseResult(key *pgp.KeyInfo, keyPath, pass string) (r Result, enabled, blocking bool) {
	switch {
	case key == nil || !key.HasPrivate:
		return neutral("Only needed with a private key."), false, false
	case !key.Locked:
		return neutral("Not needed: this private key has no passphrase."), false, false
	case pass == "":
		return neutral("Required: unlocks your private key."), true, true
	}
	err := c.checkPassphrase(keyPath, pass)
	switch {
	case err == nil:
		return ok("Passphrase is correct."), true, false
	case errors.Is(err, pgp.ErrWrongPassphrase):
		return fail("Incorrect passphrase. Check Caps Lock and keyboard layout."), true, true
	default:
		return fail("Cannot check passphrase: %v.", err), true, true
	}
}

// checkOutput validates the destination path.
func checkOutput(out, in string) Result {
	if out == "" {
		return neutral("Required: where to save the result.")
	}
	if bad := invalidNameChars(runtime.GOOS, filepath.Base(out)); bad != "" {
		return fail("File name contains characters not allowed on Windows: %s", bad)
	}
	if in != "" && samePath(out, in) {
		return fail("Must be different from the input file.")
	}
	fi, err := os.Stat(out)
	if err == nil {
		if fi.IsDir() {
			return fail("This is a folder. Add a file name at the end.")
		}
		return warn("File already exists. You will be asked before it is replaced.")
	}
	dir := filepath.Dir(out)
	di, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fail("Folder does not exist: %s", dir)
	case err != nil:
		return fail("Cannot use this folder: %v.", err)
	case !di.IsDir():
		return fail("%s is a file, not a folder.", dir)
	}
	return ok("New file will be created.")
}

func invalidNameChars(goos, name string) string {
	if goos != "windows" {
		return ""
	}
	var bad []string
	for _, r := range `<>:"|?*` {
		if strings.ContainsRune(name, r) {
			bad = append(bad, string(r))
		}
	}
	return strings.Join(bad, " ")
}

func samePath(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	ca, errA := filepath.Abs(a)
	cb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

// summary lists what still blocks the run button, in field order.
func summary(readyText string, todo []string) string {
	if len(todo) == 0 {
		return readyText
	}
	return "To continue: " + strings.Join(todo, "; ") + "."
}

// HumanBytes formats a size with binary units.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
