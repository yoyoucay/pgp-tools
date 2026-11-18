package pgp

import (
	"errors"
	"fmt"
)

// Sentinel errors. Callers match with errors.Is; the UI maps them to user-facing text.
var (
	ErrWrongPassphrase  = errors.New("passphrase is incorrect")
	ErrPassphraseNeeded = errors.New("private key is protected; passphrase required")
	ErrNotPrivateKey    = errors.New("key file contains no usable private key")
	ErrNoEncryptionKey  = errors.New("public key has no valid encryption subkey (expired, revoked, or sign-only)")
	ErrNoSigningKey     = errors.New("private key has no valid signing subkey (expired, revoked, or encrypt-only)")
	ErrNoMatchingKey    = errors.New("message is not encrypted for this private key")
	ErrSymmetricOnly    = errors.New("message is passphrase-encrypted (symmetric); not supported")
	ErrNotEncrypted     = errors.New("input is not an OpenPGP encrypted message")
	ErrIntegrity        = errors.New("message integrity check failed; data was modified or corrupted")
	ErrSignatureInvalid = errors.New("signature is invalid")
	ErrSignerMismatch   = errors.New("message is signed by a key other than the provided public key")
	ErrNotSigned        = errors.New("message is not signed but signature verification was requested")
	ErrOutputExists     = errors.New("output file already exists")
	ErrSameInputOutput  = errors.New("output path must differ from input path")
	ErrInputNotRegular  = errors.New("input is not a regular file")
	ErrCanceled         = errors.New("operation canceled")
)

// ValidationError reports a missing or malformed request field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

// KeyError reports a key file that cannot be read or parsed.
type KeyError struct {
	Path string
	Err  error
}

func (e *KeyError) Error() string { return fmt.Sprintf("key %q: %v", e.Path, e.Err) }
func (e *KeyError) Unwrap() error { return e.Err }
