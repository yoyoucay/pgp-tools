package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

func TestDescribe(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&pgp.ValidationError{Field: "public key", Reason: "required"}, "Public key required."},
		{&pgp.KeyError{Path: "/k/missing.asc", Err: fs.ErrNotExist}, "Key file not found: /k/missing.asc."},
		{&pgp.KeyError{Path: "/k/alice.asc", Err: pgp.ErrWrongPassphrase}, `Key file "alice.asc": passphrase is incorrect.`},
		{fmt.Errorf("wrap: %w", &pgp.ValidationError{Field: "input file", Reason: "required"}), "Input file required."},
		{pgp.ErrNoMatchingKey, "Message is not encrypted for this private key."},
		{fmt.Errorf("finalize output: %w", fs.ErrPermission), "Access denied. If the file is open in another program, close it and try again. Otherwise choose a folder you can write to."},
		{errors.New(""), "."},
	} {
		if got := describe(tc.err); got != tc.want {
			t.Errorf("describe(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestSignatureText(t *testing.T) {
	for _, tc := range []struct {
		res  pgp.DecryptResult
		want string
	}{
		{pgp.DecryptResult{Signature: pgp.SigNone}, "Signature: none."},
		{pgp.DecryptResult{Signature: pgp.SigVerified, SignerKeyID: "ABCD"}, "Signature: valid (key ABCD)."},
		{pgp.DecryptResult{Signature: pgp.SigUnverified, SignerKeyID: "ABCD"}, "Signature: present from key ABCD but NOT verified (no public key given)."},
	} {
		if got := signatureText(tc.res); got != tc.want {
			t.Errorf("signatureText(%+v) = %q, want %q", tc.res, got, tc.want)
		}
	}
}
