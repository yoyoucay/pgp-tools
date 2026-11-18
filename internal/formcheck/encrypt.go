package formcheck

import "github.com/yoyoucay/pgp-tools/internal/pgp"

type EncryptForm struct {
	Input, PublicKey, PrivateKey, Passphrase, Output string
}

type EncryptState struct {
	Input, PublicKey, PrivateKey, Passphrase, Output Result
	PassphraseEnabled                                bool
	Ready                                            bool
	Summary                                          string
}

// Encrypt validates the encrypt form. Paths must already be normalized.
func (c *Checker) Encrypt(f EncryptForm) EncryptState {
	var s EncryptState
	var todo []string

	if r, fi := checkReadableFile(f.Input, "Required: the file you want to protect."); fi == nil {
		s.Input = r
	} else if fi.Size() == 0 {
		s.Input = warn("File is empty. It can still be encrypted.")
	} else {
		s.Input = ok("%s, ready to encrypt.", HumanBytes(fi.Size()))
	}
	todo = appendTodo(todo, s.Input, "choose the input file", "fix the input file")

	if r, key := inspectKey(f.PublicKey, "Required: the recipient's public key (.asc, .gpg). Only they can open the result."); key == nil {
		s.PublicKey = r
	} else if !key.CanEncrypt {
		s.PublicKey = fail("This key cannot encrypt: it is expired, revoked, or for signing only.")
	} else if key.HasPrivate {
		s.PublicKey = warn("%s. This file also holds a PRIVATE key; only the public part is used. Never send it to anyone.", describeKey(key))
	} else {
		s.PublicKey = ok("Recipient: %s.", describeKey(key))
	}
	todo = appendTodo(todo, s.PublicKey, "choose the recipient's public key", "fix the public key")

	var signer *pgp.KeyInfo
	if r, key := inspectKey(f.PrivateKey, "Optional: your private key, to sign the file so the recipient knows it came from you."); key == nil {
		s.PrivateKey = r
	} else if !key.HasPrivate {
		s.PrivateKey = fail("This is a public key. Signing needs your private key. Leave empty to skip signing.")
	} else if !key.CanSign {
		s.PrivateKey = fail("This key cannot sign: it is expired, revoked, or for encryption only.")
	} else {
		signer = key
		s.PrivateKey = ok("Signed by: %s.", describeKey(key))
	}
	todo = appendTodo(todo, s.PrivateKey, "", "fix or clear the private key")

	var blocking bool
	s.Passphrase, s.PassphraseEnabled, blocking = c.passphraseResult(signer, f.PrivateKey, f.Passphrase)
	if blocking {
		todo = append(todo, passphraseTodo(s.Passphrase))
	}

	s.Output = checkOutput(f.Output, f.Input)
	todo = appendTodo(todo, s.Output, "choose where to save", "fix the output file")

	s.Ready = len(todo) == 0
	s.Summary = summary("Ready to encrypt.", todo)
	return s
}

// appendTodo adds the matching hint when r blocks the run. emptyTodo "" marks an optional field.
func appendTodo(todo []string, r Result, emptyTodo, errorTodo string) []string {
	switch {
	case r.Level == Error:
		return append(todo, errorTodo)
	case r.Level == Neutral && emptyTodo != "":
		return append(todo, emptyTodo)
	}
	return todo
}

func passphraseTodo(r Result) string {
	if r.Level == Error {
		return "fix the passphrase"
	}
	return "enter the passphrase"
}
