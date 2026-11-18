// Package pgp implements file-level OpenPGP encrypt/decrypt with optional sign/verify.
package pgp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// EncryptRequest: RecipientKey (public) is required. SignerKey (private) is optional;
// Passphrase unlocks SignerKey when it is protected.
type EncryptRequest struct {
	InputPath    string
	OutputPath   string
	RecipientKey string
	SignerKey    string
	Passphrase   []byte
	Armor        bool
	Overwrite    bool
}

// DecryptRequest: PrivateKey is required; Passphrase unlocks it when protected.
// VerifyKey (public) is optional; when set, a valid signature from it is mandatory.
type DecryptRequest struct {
	InputPath  string
	OutputPath string
	PrivateKey string
	Passphrase []byte
	VerifyKey  string
	Overwrite  bool
}

type SignatureStatus int

const (
	SigNone       SignatureStatus = iota // message carried no signature
	SigVerified                          // signature valid against VerifyKey
	SigUnverified                        // signed, but no VerifyKey supplied to check it
)

type DecryptResult struct {
	Signature   SignatureStatus
	SignerKeyID string
	Bytes       int64
}

type EncryptResult struct {
	Bytes int64
}

func (r *EncryptRequest) Validate() error {
	if err := validatePaths(r.InputPath, r.OutputPath); err != nil {
		return err
	}
	if r.RecipientKey == "" {
		return &ValidationError{Field: "public key", Reason: "required"}
	}
	if r.SignerKey == "" && len(r.Passphrase) > 0 {
		return &ValidationError{Field: "passphrase", Reason: "set only when a private key is provided for signing"}
	}
	return nil
}

func (r *DecryptRequest) Validate() error {
	if err := validatePaths(r.InputPath, r.OutputPath); err != nil {
		return err
	}
	if r.PrivateKey == "" {
		return &ValidationError{Field: "private key", Reason: "required"}
	}
	return nil
}

func validatePaths(in, out string) error {
	if in == "" {
		return &ValidationError{Field: "input file", Reason: "required"}
	}
	if out == "" {
		return &ValidationError{Field: "output file", Reason: "required"}
	}
	if samePath(in, out) {
		return ErrSameInputOutput
	}
	return nil
}

func config() *packet.Config {
	return &packet.Config{DefaultCipher: packet.CipherAES256}
}

// Encrypt encrypts InputPath to the recipient public key(s), optionally signing.
func Encrypt(ctx context.Context, req EncryptRequest) (EncryptResult, error) {
	if err := req.Validate(); err != nil {
		return EncryptResult{}, err
	}
	now := time.Now()

	to, err := loadRecipients(req.RecipientKey, now)
	if err != nil {
		return EncryptResult{}, err
	}

	var signer *openpgp.Entity
	if req.SignerKey != "" {
		keys, err := loadPrivateKeys(req.SignerKey, req.Passphrase)
		if err != nil {
			return EncryptResult{}, err
		}
		for _, e := range keys {
			if k, ok := e.SigningKey(now); ok && k.PrivateKey != nil && !k.PrivateKey.Encrypted {
				signer = e
				break
			}
		}
		if signer == nil {
			return EncryptResult{}, &KeyError{Path: req.SignerKey, Err: ErrNoSigningKey}
		}
	}

	in, err := openInput(req.InputPath)
	if err != nil {
		return EncryptResult{}, fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	out, err := createAtomic(req.OutputPath, req.Overwrite)
	if err != nil {
		return EncryptResult{}, err
	}
	defer out.Abort()

	var sink io.Writer = out
	var armorW io.WriteCloser
	if req.Armor {
		armorW, err = armor.Encode(out, "PGP MESSAGE", nil)
		if err != nil {
			return EncryptResult{}, err
		}
		sink = armorW
	}

	hints := &openpgp.FileHints{IsBinary: true, FileName: filepath.Base(req.InputPath), ModTime: now}
	pw, err := openpgp.Encrypt(sink, to, signer, hints, config())
	if err != nil {
		return EncryptResult{}, fmt.Errorf("encrypt: %w", err)
	}
	n, err := io.Copy(pw, ctxReader{ctx, in})
	if err != nil {
		_ = pw.Close()
		return EncryptResult{}, wrapIO(err)
	}
	if err := pw.Close(); err != nil {
		return EncryptResult{}, fmt.Errorf("encrypt: %w", err)
	}
	if armorW != nil {
		if err := armorW.Close(); err != nil {
			return EncryptResult{}, fmt.Errorf("armor: %w", err)
		}
	}
	if err := out.Commit(); err != nil {
		return EncryptResult{}, err
	}
	return EncryptResult{Bytes: n}, nil
}

// Decrypt decrypts InputPath (armored or binary). Output is committed only after
// integrity and, when VerifyKey is set, signature checks pass.
func Decrypt(ctx context.Context, req DecryptRequest) (DecryptResult, error) {
	if err := req.Validate(); err != nil {
		return DecryptResult{}, err
	}

	keyring, err := loadPrivateKeys(req.PrivateKey, req.Passphrase)
	if err != nil {
		return DecryptResult{}, err
	}
	var verifiers openpgp.EntityList
	if req.VerifyKey != "" {
		if verifiers, err = loadVerifiers(req.VerifyKey); err != nil {
			return DecryptResult{}, err
		}
		keyring = append(keyring, verifiers...)
	}

	in, err := openInput(req.InputPath)
	if err != nil {
		return DecryptResult{}, fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	body, err := decodeInput(ctxReader{ctx, in})
	if err != nil {
		return DecryptResult{}, wrapIO(err)
	}

	// Keys are unlocked up front, so the prompt only runs when no key matched.
	prompt := func(_ []openpgp.Key, symmetric bool) ([]byte, error) {
		if symmetric {
			return nil, ErrSymmetricOnly
		}
		return nil, ErrNoMatchingKey
	}
	md, err := openpgp.ReadMessage(body, keyring, prompt, config())
	if err != nil {
		return DecryptResult{}, mapReadError(err)
	}
	if !md.IsEncrypted {
		return DecryptResult{}, ErrNotEncrypted
	}

	out, err := createAtomic(req.OutputPath, req.Overwrite)
	if err != nil {
		return DecryptResult{}, err
	}
	defer out.Abort()

	n, err := io.Copy(out, md.UnverifiedBody)
	if err != nil {
		return DecryptResult{}, mapBodyError(err)
	}

	res := DecryptResult{Bytes: n}
	if md.IsSigned {
		res.SignerKeyID = keyIDHex(md.SignedByKeyId)
	}
	if err := checkSignature(md, verifiers, &res); err != nil {
		return DecryptResult{}, err
	}
	if err := out.Commit(); err != nil {
		return DecryptResult{}, err
	}
	return res, nil
}

func checkSignature(md *openpgp.MessageDetails, verifiers openpgp.EntityList, res *DecryptResult) error {
	if !md.IsSigned {
		if len(verifiers) > 0 {
			return ErrNotSigned
		}
		res.Signature = SigNone
		return nil
	}
	if len(verifiers) == 0 {
		res.Signature = SigUnverified
		return nil
	}
	// SignedBy may resolve to the decryption key itself since both share one keyring;
	// require the signer to be one of the explicitly supplied verification keys.
	if md.SignedBy == nil || !containsEntity(verifiers, md.SignedBy.Entity) {
		return fmt.Errorf("%w (signer key ID %s)", ErrSignerMismatch, res.SignerKeyID)
	}
	if md.SignatureError != nil {
		return fmt.Errorf("%w: %v", ErrSignatureInvalid, md.SignatureError)
	}
	res.Signature = SigVerified
	return nil
}

func containsEntity(list openpgp.EntityList, e *openpgp.Entity) bool {
	if e == nil || e.PrimaryKey == nil {
		return false
	}
	for _, v := range list {
		if v.PrimaryKey != nil && bytes.Equal(v.PrimaryKey.Fingerprint, e.PrimaryKey.Fingerprint) {
			return true
		}
	}
	return false
}

func mapReadError(err error) error {
	switch {
	case errors.Is(err, ErrCanceled), errors.Is(err, ErrSymmetricOnly), errors.Is(err, ErrNoMatchingKey):
		return err
	case errors.Is(err, pgperrors.ErrKeyIncorrect):
		return ErrNoMatchingKey
	}
	var se pgperrors.StructuralError
	var ue pgperrors.UnsupportedError
	if errors.As(err, &se) || errors.As(err, &ue) {
		return fmt.Errorf("%w: %v", ErrNotEncrypted, err)
	}
	return fmt.Errorf("read message: %w", err)
}

func mapBodyError(err error) error {
	if errors.Is(err, ErrCanceled) {
		return ErrCanceled
	}
	var (
		sigErr  pgperrors.SignatureError
		sessErr pgperrors.DecryptWithSessionKeyError
		aeadErr pgperrors.AEADError
		se      pgperrors.StructuralError
	)
	if errors.As(err, &sigErr) || errors.As(err, &sessErr) || errors.As(err, &aeadErr) || errors.As(err, &se) {
		return fmt.Errorf("%w: %v", ErrIntegrity, err)
	}
	return fmt.Errorf("decrypt: %w", err)
}

func wrapIO(err error) error {
	if errors.Is(err, ErrCanceled) {
		return ErrCanceled
	}
	return err
}
