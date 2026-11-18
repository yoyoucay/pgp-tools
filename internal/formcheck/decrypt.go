package formcheck

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

type DecryptForm struct {
	Input, PrivateKey, Passphrase, PublicKey, Output string
}

type DecryptState struct {
	Input, PrivateKey, Passphrase, PublicKey, Output Result
	PassphraseEnabled                                bool
	Ready                                            bool
	Summary                                          string
}

// Decrypt validates the decrypt form. Paths must already be normalized.
func (c *Checker) Decrypt(f DecryptForm) DecryptState {
	var s DecryptState
	var todo []string

	var msg *pgp.MessageInfo
	if r, fi := checkReadableFile(f.Input, "Required: the encrypted file (.gpg, .pgp, .asc)."); fi == nil {
		s.Input = r
	} else if m, err := pgp.InspectMessage(f.Input); err != nil {
		if errors.Is(err, pgp.ErrNotEncrypted) {
			s.Input = fail("Not an encrypted OpenPGP file. To protect this file, use Encrypt.")
		} else {
			s.Input = fail("Cannot read this file: %v.", err)
		}
	} else if len(m.Recipients) == 0 {
		s.Input = fail("This file is protected with a password only. That is not supported; ask the sender to encrypt to your public key.")
	} else {
		msg = &m
		s.Input = ok("Encrypted file, %s, for %s.", HumanBytes(fi.Size()), plural(len(m.Recipients), "recipient"))
	}
	todo = appendTodo(todo, s.Input, "choose the encrypted file", "fix the input file")

	var key *pgp.KeyInfo
	if r, k := inspectKey(f.PrivateKey, "Required: your private key (.asc, .gpg, .key)."); k == nil {
		s.PrivateKey = r
	} else if !k.HasPrivate {
		s.PrivateKey = fail("This is a public key. Decrypting needs YOUR private key, the one that pairs with the public key you gave the sender.")
	} else if msg != nil && !k.Matches(*msg) {
		s.PrivateKey = fail("This file was not encrypted for this key. It is for key ID %s.", strings.Join(msg.Recipients, ", "))
	} else {
		key = k
		if msg != nil {
			s.PrivateKey = ok("%s. Matches this file.", describeKey(k))
		} else {
			s.PrivateKey = ok("%s.", describeKey(k))
		}
	}
	todo = appendTodo(todo, s.PrivateKey, "choose your private key", "fix the private key")

	var blocking bool
	s.Passphrase, s.PassphraseEnabled, blocking = c.passphraseResult(key, f.PrivateKey, f.Passphrase)
	if blocking {
		todo = append(todo, passphraseTodo(s.Passphrase))
	}

	if r, k := inspectKey(f.PublicKey, "Optional: the sender's public key, to confirm who signed the file."); k == nil {
		s.PublicKey = r
	} else {
		s.PublicKey = ok("Signature from %s will be required.", describeKey(k))
	}
	todo = appendTodo(todo, s.PublicKey, "", "fix or clear the public key")

	s.Output = checkOutput(f.Output, f.Input)
	todo = appendTodo(todo, s.Output, "choose where to save", "fix the output file")

	s.Ready = len(todo) == 0
	s.Summary = summary("Ready to decrypt.", todo)
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
