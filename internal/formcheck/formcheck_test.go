package formcheck

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

type keys struct{ pub, priv string }

type env struct {
	dir               string
	plain, empty, enc string
	alice, bob        keys // alice: passphrase-protected; bob: unprotected
	alicePass         string
}

func writeKey(t *testing.T, dir, name, pass string) keys {
	t.Helper()
	e, err := openpgp.NewEntity(name, "", name+"@example.test", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatal(err)
	}
	var pub, priv bytes.Buffer
	w, _ := armor.Encode(&pub, openpgp.PublicKeyType, nil)
	e.Serialize(w)
	w.Close()
	if pass != "" {
		if err := e.EncryptPrivateKeys([]byte(pass), nil); err != nil {
			t.Fatal(err)
		}
	}
	w, _ = armor.Encode(&priv, openpgp.PrivateKeyType, nil)
	e.SerializePrivateWithoutSigning(w, nil)
	w.Close()
	k := keys{filepath.Join(dir, name+".pub.asc"), filepath.Join(dir, name+".sec.asc")}
	must(t, os.WriteFile(k.pub, pub.Bytes(), 0o600))
	must(t, os.WriteFile(k.priv, priv.Bytes(), 0o600))
	return k
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newEnv(t *testing.T) env {
	t.Helper()
	dir := t.TempDir()
	e := env{dir: dir, alicePass: "correct horse"}
	e.alice = writeKey(t, dir, "alice", e.alicePass)
	e.bob = writeKey(t, dir, "bob", "")
	e.plain = filepath.Join(dir, "report.txt")
	must(t, os.WriteFile(e.plain, []byte("hello"), 0o600))
	e.empty = filepath.Join(dir, "empty.txt")
	must(t, os.WriteFile(e.empty, nil, 0o600))
	e.enc = filepath.Join(dir, "report.txt.gpg")
	if _, err := pgp.Encrypt(context.Background(), pgp.EncryptRequest{InputPath: e.plain, OutputPath: e.enc, RecipientKey: e.bob.pub}); err != nil {
		t.Fatal(err)
	}
	return e
}

func expect(t *testing.T, field string, got Result, level Level, contains string) {
	t.Helper()
	if got.Level != level || !strings.Contains(got.Text, contains) {
		t.Errorf("%s = {%d %q}, want level %d containing %q", field, got.Level, got.Text, level, contains)
	}
}

func TestEncryptEmptyForm(t *testing.T) {
	s := NewChecker().Encrypt(EncryptForm{})
	if s.Ready || s.PassphraseEnabled {
		t.Fatal("empty form must not be ready; passphrase must be disabled")
	}
	for name, r := range map[string]Result{"input": s.Input, "public": s.PublicKey, "private": s.PrivateKey, "pass": s.Passphrase, "output": s.Output} {
		if r.Level != Neutral {
			t.Errorf("%s: untouched field must be neutral, got %+v", name, r)
		}
	}
	want := "To continue: choose the input file; choose the recipient's public key; choose where to save."
	if s.Summary != want {
		t.Fatalf("summary = %q, want %q", s.Summary, want)
	}
}

func TestEncryptReady(t *testing.T) {
	e := newEnv(t)
	c := NewChecker()
	f := EncryptForm{Input: e.plain, PublicKey: e.bob.pub, Output: filepath.Join(e.dir, "out.gpg")}
	s := c.Encrypt(f)
	if !s.Ready || s.Summary != "Ready to encrypt." {
		t.Fatalf("state = %+v", s)
	}
	expect(t, "public", s.PublicKey, OK, "bob <bob@example.test>")

	// Signing with a protected key: passphrase becomes required, then validated.
	f.PrivateKey = e.alice.priv
	s = c.Encrypt(f)
	if s.Ready || !s.PassphraseEnabled {
		t.Fatalf("protected signer without passphrase: ready=%v enabled=%v", s.Ready, s.PassphraseEnabled)
	}
	expect(t, "pass", s.Passphrase, Neutral, "Required")
	if !strings.Contains(s.Summary, "enter the passphrase") {
		t.Fatalf("summary = %q", s.Summary)
	}

	f.Passphrase = "wrong"
	s = c.Encrypt(f)
	expect(t, "pass", s.Passphrase, Error, "Incorrect passphrase")
	if s.Ready || !strings.Contains(s.Summary, "fix the passphrase") {
		t.Fatalf("wrong passphrase: ready=%v summary=%q", s.Ready, s.Summary)
	}

	f.Passphrase = e.alicePass
	s = c.Encrypt(f)
	expect(t, "pass", s.Passphrase, OK, "correct")
	if !s.Ready {
		t.Fatalf("correct passphrase should be ready: %q", s.Summary)
	}

	// Unprotected signer: passphrase not needed.
	f.PrivateKey, f.Passphrase = e.bob.priv, ""
	s = c.Encrypt(f)
	if s.PassphraseEnabled || !s.Ready {
		t.Fatalf("unprotected signer: enabled=%v ready=%v", s.PassphraseEnabled, s.Ready)
	}
}

func TestEncryptFieldErrors(t *testing.T) {
	e := newEnv(t)
	c := NewChecker()
	s := c.Encrypt(EncryptForm{
		Input:      filepath.Join(e.dir, "missing.txt"),
		PublicKey:  e.plain,
		PrivateKey: e.bob.pub,
		Output:     filepath.Join(e.dir, "no-such-dir", "out.gpg"),
	})
	expect(t, "input", s.Input, Error, "File not found")
	expect(t, "public", s.PublicKey, Error, "Not an OpenPGP key file")
	expect(t, "private", s.PrivateKey, Error, "This is a public key")
	expect(t, "output", s.Output, Error, "Folder does not exist")
	if s.Ready {
		t.Fatal("must not be ready")
	}

	s = c.Encrypt(EncryptForm{Input: e.dir, PublicKey: e.alice.priv, Output: e.plain})
	expect(t, "input", s.Input, Error, "This is a folder")
	expect(t, "public", s.PublicKey, Warn, "PRIVATE key")
	expect(t, "output", s.Output, Warn, "already exists")

	s = c.Encrypt(EncryptForm{Input: e.empty, Output: e.empty})
	expect(t, "input", s.Input, Warn, "empty")
	expect(t, "output", s.Output, Error, "different from the input")

	s = c.Encrypt(EncryptForm{Input: e.plain, Output: e.dir})
	expect(t, "output", s.Output, Error, "This is a folder")
}

func TestDecrypt(t *testing.T) {
	e := newEnv(t)
	c := NewChecker()
	out := filepath.Join(e.dir, "report.out.txt")

	s := c.Decrypt(DecryptForm{Input: e.enc, PrivateKey: e.bob.priv, Output: out})
	if !s.Ready {
		t.Fatalf("not ready: %q", s.Summary)
	}
	expect(t, "input", s.Input, OK, "1 recipient")
	expect(t, "private", s.PrivateKey, OK, "Matches this file")

	s = c.Decrypt(DecryptForm{Input: e.enc, PrivateKey: e.alice.priv, Passphrase: e.alicePass, Output: out})
	expect(t, "private", s.PrivateKey, Error, "not encrypted for this key")
	if s.PassphraseEnabled {
		t.Fatal("passphrase must stay disabled for a key that cannot be used")
	}

	s = c.Decrypt(DecryptForm{Input: e.plain, PrivateKey: e.bob.pub, PublicKey: e.plain, Output: out})
	expect(t, "input", s.Input, Error, "Not an encrypted OpenPGP file")
	expect(t, "private", s.PrivateKey, Error, "This is a public key")
	expect(t, "public", s.PublicKey, Error, "Not an OpenPGP key file")

	s = c.Decrypt(DecryptForm{Input: e.enc, PrivateKey: e.bob.priv, PublicKey: e.alice.pub, Output: out})
	expect(t, "public", s.PublicKey, OK, "alice")
	if !s.Ready {
		t.Fatalf("not ready: %q", s.Summary)
	}

	s = c.Decrypt(DecryptForm{})
	want := "To continue: choose the encrypted file; choose your private key; choose where to save."
	if s.Summary != want {
		t.Fatalf("summary = %q, want %q", s.Summary, want)
	}
}

func TestPassphraseCacheInvalidatesOnKeyChange(t *testing.T) {
	e := newEnv(t)
	c := NewChecker()
	if err := c.checkPassphrase(e.alice.priv, e.alicePass); err != nil {
		t.Fatal(err)
	}
	// Replace the key file with a different protected key: cached "correct" must not leak.
	other := writeKey(t, t.TempDir(), "carol", "different-pass-longer")
	data, _ := os.ReadFile(other.priv)
	must(t, os.WriteFile(e.alice.priv, data, 0o600))
	if err := c.checkPassphrase(e.alice.priv, e.alicePass); err == nil {
		t.Fatal("stale cache entry reused after key file changed")
	}
}

func TestNormalizePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	sep := string(filepath.Separator)
	for in, want := range map[string]string{
		"":                        "",
		"   ":                     "",
		`""`:                      "",
		`  "a b/c.txt"  `:         filepath.Join("a b", "c.txt"),
		`'x.asc'`:                 "x.asc",
		"dir/../file.gpg":         "file.gpg",
		"~/keys/k.asc":            filepath.Join(home, "keys", "k.asc"),
		"a" + sep + sep + "b.txt": filepath.Join("a", "b.txt"),
	} {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInvalidNameChars(t *testing.T) {
	if got := invalidNameChars("windows", `bad<na|me?.txt`); got != "< | ?" {
		t.Errorf("windows = %q", got)
	}
	if got := invalidNameChars("linux", `bad<na|me?.txt`); got != "" {
		t.Errorf("linux = %q", got)
	}
}

func TestOutputWindowsNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only file name rules")
	}
	r := checkOutput(filepath.Join(t.TempDir(), "a?b.txt"), "")
	expect(t, "output", r, Error, "not allowed on Windows")
}

func TestRoute(t *testing.T) {
	for _, tc := range []struct {
		kind    pgp.FileKind
		current Tab
		tab     Tab
		field   Field
	}{
		{pgp.KindPublicKey, TabEncrypt, TabEncrypt, FieldPublicKey},
		{pgp.KindPublicKey, TabDecrypt, TabDecrypt, FieldPublicKey},
		{pgp.KindPrivateKey, TabDecrypt, TabDecrypt, FieldPrivateKey},
		{pgp.KindMessage, TabEncrypt, TabDecrypt, FieldInput},
		{pgp.KindOther, TabDecrypt, TabEncrypt, FieldInput},
	} {
		tab, field := Route(tc.kind, tc.current)
		if tab != tc.tab || field != tc.field {
			t.Errorf("Route(%v, %v) = %v,%v want %v,%v", tc.kind, tc.current, tab, field, tc.tab, tc.field)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 5 << 20: "5.0 MiB", 3 << 30: "3.0 GiB"} {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
