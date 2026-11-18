package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

// describe turns an error into a single user-facing sentence.
func describe(err error) string {
	var ve *pgp.ValidationError
	if errors.As(err, &ve) {
		return capitalize(ve.Field) + " " + ve.Reason + "."
	}
	var ke *pgp.KeyError
	if errors.As(err, &ke) {
		if errors.Is(ke.Err, fs.ErrNotExist) {
			return fmt.Sprintf("Key file not found: %s.", ke.Path)
		}
		return fmt.Sprintf("Key file %q: %v.", filepath.Base(ke.Path), ke.Err)
	}
	return capitalize(err.Error()) + "."
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func humanBytes(n int64) string {
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

func signatureText(r pgp.DecryptResult) string {
	switch r.Signature {
	case pgp.SigVerified:
		return "Signature: valid (key " + r.SignerKeyID + ")."
	case pgp.SigUnverified:
		return "Signature: present from key " + r.SignerKeyID + " but NOT verified (no public key given)."
	default:
		return "Signature: none."
	}
}
