package pgp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, ErrCanceled
	}
	return c.r.Read(p)
}

// atomicFile writes to a hidden temp file beside the target and renames on Commit,
// so a failed or canceled run never leaves partial plaintext/ciphertext behind.
type atomicFile struct {
	*os.File
	target    string
	overwrite bool
	done      bool
}

func createAtomic(target string, overwrite bool) (*atomicFile, error) {
	if !overwrite {
		if _, err := os.Lstat(target); err == nil {
			return nil, ErrOutputExists
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	f, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create output: %w", err)
	}
	return &atomicFile{File: f, target: target, overwrite: overwrite}, nil
}

func (a *atomicFile) Commit() error {
	if err := a.Sync(); err != nil {
		a.Abort()
		return fmt.Errorf("flush output: %w", err)
	}
	if err := a.Close(); err != nil {
		a.Abort()
		return fmt.Errorf("close output: %w", err)
	}
	if !a.overwrite {
		// Re-check: another process may have created the target meanwhile.
		if _, err := os.Lstat(a.target); err == nil {
			a.Abort()
			return ErrOutputExists
		}
	}
	if err := os.Rename(a.Name(), a.target); err != nil {
		a.Abort()
		return fmt.Errorf("finalize output: %w", err)
	}
	a.done = true
	return nil
}

// Abort is safe to call after Commit; it then does nothing.
func (a *atomicFile) Abort() {
	if a.done {
		return
	}
	a.done = true
	_ = a.Close()
	_ = os.Remove(a.Name())
}

func openInput(path string) (*os.File, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, ErrInputNotRegular
	}
	return os.Open(path)
}

func samePath(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	ca, errA := filepath.Abs(a)
	cb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && ca == cb
}

// DefaultEncryptOutput appends .asc (armored) or .gpg (binary).
func DefaultEncryptOutput(input string, armored bool) string {
	if input == "" {
		return ""
	}
	if armored {
		return input + ".asc"
	}
	return input + ".gpg"
}

// DefaultDecryptOutput strips a known OpenPGP extension, else appends .decrypted.
func DefaultDecryptOutput(input string) string {
	if input == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(input))
	switch ext {
	case ".gpg", ".pgp", ".asc":
		base := input[:len(input)-len(ext)]
		if base != "" && !strings.HasSuffix(base, string(filepath.Separator)) && !strings.HasSuffix(base, "/") {
			return base
		}
	}
	return input + ".decrypted"
}
