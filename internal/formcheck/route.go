package formcheck

import "github.com/yoyoucay/pgp-tools/internal/pgp"

type Tab int

const (
	TabEncrypt Tab = iota
	TabDecrypt
)

type Field int

const (
	FieldInput Field = iota
	FieldPublicKey
	FieldPrivateKey
)

// Route decides where a dropped file goes. Keys stay on the current tab; an
// encrypted file always goes to Decrypt and any other file to Encrypt.
func Route(kind pgp.FileKind, current Tab) (Tab, Field) {
	switch kind {
	case pgp.KindPublicKey:
		return current, FieldPublicKey
	case pgp.KindPrivateKey:
		return current, FieldPrivateKey
	case pgp.KindMessage:
		return TabDecrypt, FieldInput
	default:
		return TabEncrypt, FieldInput
	}
}
