package pgp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// KeyInfo describes a key file without unlocking it.
type KeyInfo struct {
	UserID     string // primary identity of the first key, e.g. "Alice <alice@example.org>"
	KeyID      string // primary key ID of the first key, 16 hex digits
	Count      int    // number of keys in the file
	HasPrivate bool   // file carries secret key material
	Locked     bool   // secret material is passphrase-protected
	CanEncrypt bool   // at least one key has a valid encryption subkey
	CanSign    bool   // at least one private key has a valid signing key

	// secretIDs are key IDs (primary and subkeys) with secret material, used to
	// match a message's recipients before attempting decryption.
	secretIDs map[uint64]bool
}

// InspectKey parses a key file and reports its capabilities.
func InspectKey(path string) (KeyInfo, error) {
	list, err := readKeyFile(path)
	if err != nil {
		return KeyInfo{}, err
	}
	now := time.Now()
	info := KeyInfo{Count: len(list), secretIDs: map[uint64]bool{}}
	if first := list[0]; first.PrimaryKey != nil {
		info.KeyID = keyIDHex(first.PrimaryKey.KeyId)
		if id := first.PrimaryIdentity(); id != nil {
			info.UserID = id.Name
		}
	}
	for _, e := range list {
		if _, ok := e.EncryptionKey(now); ok {
			info.CanEncrypt = true
		}
		if !hasSecret(e) {
			continue
		}
		info.HasPrivate = true
		if isLocked(e) {
			info.Locked = true
		}
		if k, ok := e.SigningKey(now); ok && k.PrivateKey != nil && !k.PrivateKey.Dummy() {
			info.CanSign = true
		}
		if e.PrivateKey != nil && !e.PrivateKey.Dummy() {
			info.secretIDs[e.PrivateKey.KeyId] = true
		}
		for _, s := range e.Subkeys {
			if s.PrivateKey != nil && !s.PrivateKey.Dummy() {
				info.secretIDs[s.PrivateKey.KeyId] = true
			}
		}
	}
	return info, nil
}

// CheckPassphrase reports whether passphrase unlocks the private key file.
// Returns ErrPassphraseNeeded, ErrWrongPassphrase, or ErrNotPrivateKey (wrapped in KeyError).
func CheckPassphrase(path string, passphrase []byte) error {
	_, err := loadPrivateKeys(path, passphrase)
	return err
}

// MessageInfo describes an encrypted file without decrypting it.
type MessageInfo struct {
	Armored    bool
	Recipients []string // recipient key IDs; "0000000000000000" marks a hidden recipient
	Symmetric  bool     // can be opened with a password
}

// maxHeaderPackets bounds the scan for session-key packets before the encrypted body.
const maxHeaderPackets = 256

// InspectMessage reads only the packet headers that precede the encrypted body.
func InspectMessage(path string) (MessageInfo, error) {
	f, err := openInput(path)
	if err != nil {
		return MessageInfo{}, err
	}
	defer f.Close()

	head := make([]byte, 1024)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return MessageInfo{}, err
	}
	info := MessageInfo{Armored: isArmored(stripBOM(head[:n]))}

	body, err := decodeInput(f)
	if err != nil {
		return MessageInfo{}, err
	}
	r := packet.NewReader(body)
	for i := 0; i < maxHeaderPackets; i++ {
		p, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return MessageInfo{}, fmt.Errorf("%w: %v", ErrNotEncrypted, err)
		}
		switch p := p.(type) {
		case *packet.EncryptedKey:
			info.Recipients = append(info.Recipients, keyIDHex(p.KeyId))
		case *packet.SymmetricKeyEncrypted:
			info.Symmetric = true
		case *packet.SymmetricallyEncrypted, *packet.AEADEncrypted:
			if len(info.Recipients) == 0 && !info.Symmetric {
				return MessageInfo{}, fmt.Errorf("%w: no recipients", ErrNotEncrypted)
			}
			return info, nil
		default:
			return MessageInfo{}, ErrNotEncrypted
		}
	}
	return MessageInfo{}, ErrNotEncrypted
}

// Matches reports whether the key can likely decrypt m. Hidden recipients
// (key ID 0) cannot be matched in advance, so they count as a possible match.
func (k KeyInfo) Matches(m MessageInfo) bool {
	for _, r := range m.Recipients {
		if r == keyIDHex(0) {
			return true
		}
		for id := range k.secretIDs {
			if keyIDHex(id) == r {
				return true
			}
		}
	}
	return false
}

// FileKind classifies a file for drag-and-drop routing.
type FileKind int

const (
	KindOther FileKind = iota
	KindPublicKey
	KindPrivateKey
	KindMessage
)

// Classify inspects a file and reports what it most likely is.
func Classify(path string) FileKind {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return KindOther
	}
	if fi.Size() <= maxKeyFileSize {
		if info, err := InspectKey(path); err == nil {
			if info.HasPrivate {
				return KindPrivateKey
			}
			return KindPublicKey
		}
	}
	if _, err := InspectMessage(path); err == nil {
		return KindMessage
	}
	return KindOther
}
