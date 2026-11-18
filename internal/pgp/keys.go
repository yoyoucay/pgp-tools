package pgp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// maxKeyFileSize guards against a data file being selected as a key by mistake.
const maxKeyFileSize = 16 << 20

var errKeyFileTooLarge = errors.New("file exceeds 16 MiB; not a key file")

func readKeyFile(path string) (openpgp.EntityList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &KeyError{Path: path, Err: err}
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxKeyFileSize+1))
	if err != nil {
		return nil, &KeyError{Path: path, Err: err}
	}
	if len(data) > maxKeyFileSize {
		return nil, &KeyError{Path: path, Err: errKeyFileTooLarge}
	}
	data = stripBOM(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, &KeyError{Path: path, Err: errors.New("file is empty")}
	}

	var list openpgp.EntityList
	if isArmored(data) {
		list, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	} else {
		list, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil {
		return nil, &KeyError{Path: path, Err: fmt.Errorf("not a valid OpenPGP key: %w", err)}
	}
	if len(list) == 0 {
		return nil, &KeyError{Path: path, Err: errors.New("no keys found")}
	}
	return list, nil
}

// loadRecipients returns entities usable as encryption recipients.
// A private key file is accepted because it carries the public half.
func loadRecipients(path string, now time.Time) (openpgp.EntityList, error) {
	list, err := readKeyFile(path)
	if err != nil {
		return nil, err
	}
	var out openpgp.EntityList
	for _, e := range list {
		if _, ok := e.EncryptionKey(now); ok {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, &KeyError{Path: path, Err: ErrNoEncryptionKey}
	}
	return out, nil
}

func loadVerifiers(path string) (openpgp.EntityList, error) {
	return readKeyFile(path)
}

// loadPrivateKeys returns entities holding secret material, unlocked with passphrase.
func loadPrivateKeys(path string, passphrase []byte) (openpgp.EntityList, error) {
	list, err := readKeyFile(path)
	if err != nil {
		return nil, err
	}
	var out openpgp.EntityList
	for _, e := range list {
		if !hasSecret(e) {
			continue
		}
		if isLocked(e) {
			if len(passphrase) == 0 {
				return nil, &KeyError{Path: path, Err: ErrPassphraseNeeded}
			}
			// The library reports a bad passphrase as either ErrKeyIncorrect or a
			// checksum StructuralError depending on S2K mode; both mean the same here.
			if err := e.DecryptPrivateKeys(passphrase); err != nil {
				return nil, &KeyError{Path: path, Err: fmt.Errorf("%w (%v)", ErrWrongPassphrase, err)}
			}
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, &KeyError{Path: path, Err: ErrNotPrivateKey}
	}
	return out, nil
}

func hasSecret(e *openpgp.Entity) bool {
	if e.PrivateKey != nil && !e.PrivateKey.Dummy() {
		return true
	}
	for _, s := range e.Subkeys {
		if s.PrivateKey != nil && !s.PrivateKey.Dummy() {
			return true
		}
	}
	return false
}

func isLocked(e *openpgp.Entity) bool {
	if e.PrivateKey != nil && !e.PrivateKey.Dummy() && e.PrivateKey.Encrypted {
		return true
	}
	for _, s := range e.Subkeys {
		if s.PrivateKey != nil && !s.PrivateKey.Dummy() && s.PrivateKey.Encrypted {
			return true
		}
	}
	return false
}

func isArmored(data []byte) bool {
	return bytes.Contains(data[:min(len(data), 1024)], []byte("-----BEGIN PGP"))
}

// decodeInput unwraps ASCII armor when present; binary input passes through.
func decodeInput(r io.Reader) (io.Reader, error) {
	head := make([]byte, 1024)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = stripBOM(head[:n])
	if len(head) == 0 {
		return nil, ErrNotEncrypted
	}
	full := io.MultiReader(bytes.NewReader(head), r)
	if !isArmored(head) {
		return full, nil
	}
	block, err := armor.Decode(full)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid armor: %v", ErrNotEncrypted, err)
	}
	if block.Type != "PGP MESSAGE" {
		return nil, fmt.Errorf("%w: armor type is %q", ErrNotEncrypted, block.Type)
	}
	return block.Body, nil
}

// stripBOM drops a UTF-8 byte order mark, which Windows Notepad adds when saving
// a pasted armored key; the armor parser would otherwise miss the BEGIN line.
func stripBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
}

func keyIDHex(id uint64) string { return fmt.Sprintf("%016X", id) }
