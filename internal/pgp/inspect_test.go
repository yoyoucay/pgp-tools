package pgp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func TestInspectKey(t *testing.T) {
	f := setup(t)

	pub, err := InspectKey(f.alice.pub)
	if err != nil {
		t.Fatal(err)
	}
	if pub.HasPrivate || !pub.CanEncrypt || pub.CanSign || pub.UserID != "alice <alice@example.test>" || len(pub.KeyID) != 16 {
		t.Fatalf("public key info = %+v", pub)
	}

	locked, err := InspectKey(f.alice.priv)
	if err != nil {
		t.Fatal(err)
	}
	if !locked.HasPrivate || !locked.Locked || !locked.CanSign {
		t.Fatalf("locked private key info = %+v", locked)
	}

	open, err := InspectKey(f.bob.priv)
	if err != nil {
		t.Fatal(err)
	}
	if !open.HasPrivate || open.Locked {
		t.Fatalf("unprotected private key info = %+v", open)
	}

	var ke *KeyError
	if _, err := InspectKey(f.plain); !errors.As(err, &ke) {
		t.Fatalf("plain file err = %v, want KeyError", err)
	}
}

func TestCheckPassphrase(t *testing.T) {
	f := setup(t)
	for _, tc := range []struct {
		name string
		path string
		pass string
		want error
	}{
		{"correct", f.alice.priv, "alice-secret", nil},
		{"wrong", f.alice.priv, "nope", ErrWrongPassphrase},
		{"empty", f.alice.priv, "", ErrPassphraseNeeded},
		{"unprotected key ignores passphrase", f.bob.priv, "anything", nil},
		{"public key", f.alice.pub, "x", ErrNotPrivateKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckPassphrase(tc.path, []byte(tc.pass)); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestInspectMessageAndMatch(t *testing.T) {
	f := setup(t)
	for _, armored := range []bool{false, true} {
		enc := f.encryptToBob(t, armored, false)
		msg, err := InspectMessage(enc)
		if err != nil {
			t.Fatalf("armored=%v: %v", armored, err)
		}
		if msg.Armored != armored || len(msg.Recipients) != 1 || msg.Symmetric {
			t.Fatalf("armored=%v: info = %+v", armored, msg)
		}
		bob, _ := InspectKey(f.bob.priv)
		alice, _ := InspectKey(f.alice.priv)
		if !bob.Matches(msg) || alice.Matches(msg) {
			t.Fatalf("match: bob=%v alice=%v", bob.Matches(msg), alice.Matches(msg))
		}
	}

	hidden := MessageInfo{Recipients: []string{keyIDHex(0)}}
	alice, _ := InspectKey(f.alice.priv)
	if !alice.Matches(hidden) {
		t.Fatal("hidden recipient must count as a possible match")
	}

	for _, path := range []string{f.plain, f.alice.pub} {
		if _, err := InspectMessage(path); !errors.Is(err, ErrNotEncrypted) {
			t.Fatalf("%s: err = %v, want ErrNotEncrypted", filepath.Base(path), err)
		}
	}
}

func TestInspectSymmetricMessage(t *testing.T) {
	f := setup(t)
	sym := filepath.Join(f.dir, "sym.gpg")
	fh, _ := os.Create(sym)
	w, _ := openpgp.SymmetricallyEncrypt(fh, []byte("pw"), nil, nil)
	w.Write([]byte("x"))
	w.Close()
	fh.Close()
	msg, err := InspectMessage(sym)
	if err != nil || !msg.Symmetric || len(msg.Recipients) != 0 {
		t.Fatalf("info = %+v err = %v", msg, err)
	}
}

func TestClassify(t *testing.T) {
	f := setup(t)
	enc := f.encryptToBob(t, true, false)
	for path, want := range map[string]FileKind{
		f.alice.pub:  KindPublicKey,
		f.alice.priv: KindPrivateKey,
		enc:          KindMessage,
		f.plain:      KindOther,
		f.dir:        KindOther,
		f.dir + "/x": KindOther,
	} {
		if got := Classify(path); got != want {
			t.Errorf("Classify(%s) = %v, want %v", filepath.Base(path), got, want)
		}
	}
}

// Windows editors save with CRLF line endings and Notepad may prepend a UTF-8 BOM.
func TestWindowsTextQuirks(t *testing.T) {
	f := setup(t)
	toWindows := func(src, dst string) string {
		data, _ := os.ReadFile(src)
		data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
		data = append([]byte("\xEF\xBB\xBF"), data...)
		writeFile(t, dst, data)
		return dst
	}
	pub := toWindows(f.bob.pub, filepath.Join(f.dir, "bob-win.pub.asc"))
	priv := toWindows(f.bob.priv, filepath.Join(f.dir, "bob-win.sec.asc"))

	enc := filepath.Join(f.dir, "win.asc")
	if _, err := Encrypt(context.Background(), EncryptRequest{InputPath: f.plain, OutputPath: enc, RecipientKey: pub, Armor: true}); err != nil {
		t.Fatalf("encrypt with CRLF+BOM key: %v", err)
	}
	encWin := toWindows(enc, filepath.Join(f.dir, "win-crlf.asc"))

	if _, err := InspectMessage(encWin); err != nil {
		t.Fatalf("inspect CRLF+BOM message: %v", err)
	}
	out := filepath.Join(f.dir, "win.txt")
	if _, err := Decrypt(context.Background(), DecryptRequest{InputPath: encWin, OutputPath: out, PrivateKey: priv}); err != nil {
		t.Fatalf("decrypt CRLF+BOM message: %v", err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, f.content) {
		t.Fatal("plaintext mismatch")
	}
}
