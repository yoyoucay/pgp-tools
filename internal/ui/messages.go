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
	if errors.Is(err, fs.ErrPermission) {
		// Windows reports a file held open by another program as access denied.
		return "Access denied. If the file is open in another program, close it and try again. Otherwise choose a folder you can write to."
	}
	return capitalize(err.Error()) + "."
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
