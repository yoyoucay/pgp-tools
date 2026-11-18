package pgp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type keyPair struct {
	pub, priv string
	entity    *openpgp.Entity
}

func newKeyPair(t *testing.T, dir, name string, passphrase []byte) keyPair {
	t.Helper()
	e, err := openpgp.NewEntity(name, "", name+"@example.test", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatal(err)
	}
	var pub bytes.Buffer
	w, _ := armor.Encode(&pub, openpgp.PublicKeyType, nil)
	if err := e.Serialize(w); err != nil {
		t.Fatal(err)
	}
	w.Close()

	if len(passphrase) > 0 {
		if err := e.EncryptPrivateKeys(passphrase, nil); err != nil {
			t.Fatal(err)
		}
	}
	var priv bytes.Buffer
	w, _ = armor.Encode(&priv, openpgp.PrivateKeyType, nil)
	if err := e.SerializePrivateWithoutSigning(w, nil); err != nil {
		t.Fatal(err)
	}
	w.Close()

	kp := keyPair{pub: filepath.Join(dir, name+".pub.asc"), priv: filepath.Join(dir, name+".sec.asc"), entity: e}
	writeFile(t, kp.pub, pub.Bytes())
	writeFile(t, kp.priv, priv.Bytes())
	return kp
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	dir        string
	alice, bob keyPair
	plain      string
	content    []byte
	alicePass  []byte
}

func setup(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	f := fixture{dir: dir, alicePass: []byte("alice-secret")}
	f.alice = newKeyPair(t, dir, "alice", f.alicePass)
	f.bob = newKeyPair(t, dir, "bob", nil)
	f.content = bytes.Repeat([]byte("payload line\n"), 5000)
	f.plain = filepath.Join(dir, "report.txt")
	writeFile(t, f.plain, f.content)
	return f
}

func (f fixture) encryptToBob(t *testing.T, armored, sign bool) string {
	t.Helper()
	req := EncryptRequest{InputPath: f.plain, OutputPath: DefaultEncryptOutput(f.plain, armored), RecipientKey: f.bob.pub, Armor: armored, Overwrite: true}
	if sign {
		req.SignerKey, req.Passphrase = f.alice.priv, f.alicePass
	}
	if _, err := Encrypt(context.Background(), req); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return req.OutputPath
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name          string
		armored, sign bool
		verify        bool
		want          SignatureStatus
	}{
		{"binary unsigned", false, false, false, SigNone},
		{"armored unsigned", true, false, false, SigNone},
		{"signed verified", true, true, true, SigVerified},
		{"signed not verified", false, true, false, SigUnverified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			enc := f.encryptToBob(t, tc.armored, tc.sign)
			out := filepath.Join(f.dir, "out.txt")
			req := DecryptRequest{InputPath: enc, OutputPath: out, PrivateKey: f.bob.priv}
			if tc.verify {
				req.VerifyKey = f.alice.pub
			}
			res, err := Decrypt(context.Background(), req)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if res.Signature != tc.want {
				t.Fatalf("signature = %v, want %v", res.Signature, tc.want)
			}
			got, _ := os.ReadFile(out)
			if !bytes.Equal(got, f.content) {
				t.Fatal("plaintext mismatch")
			}
		})
	}
}

func TestEncryptErrors(t *testing.T) {
	f := setup(t)
	out := filepath.Join(f.dir, "x.gpg")
	cases := []struct {
		name string
		req  EncryptRequest
		want error
	}{
		{"wrong passphrase", EncryptRequest{InputPath: f.plain, OutputPath: out, RecipientKey: f.bob.pub, SignerKey: f.alice.priv, Passphrase: []byte("nope")}, ErrWrongPassphrase},
		{"missing passphrase", EncryptRequest{InputPath: f.plain, OutputPath: out, RecipientKey: f.bob.pub, SignerKey: f.alice.priv}, ErrPassphraseNeeded},
		{"public key as signer", EncryptRequest{InputPath: f.plain, OutputPath: out, RecipientKey: f.bob.pub, SignerKey: f.alice.pub}, ErrNotPrivateKey},
		{"same input output", EncryptRequest{InputPath: f.plain, OutputPath: f.plain, RecipientKey: f.bob.pub}, ErrSameInputOutput},
		{"output exists", EncryptRequest{InputPath: f.plain, OutputPath: f.alice.pub, RecipientKey: f.bob.pub}, ErrOutputExists},
		{"input is directory", EncryptRequest{InputPath: f.dir, OutputPath: out, RecipientKey: f.bob.pub}, ErrInputNotRegular},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encrypt(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if _, statErr := os.Stat(out); statErr == nil {
				t.Fatal("output must not exist after failure")
			}
		})
	}

	t.Run("validation", func(t *testing.T) {
		var ve *ValidationError
		for _, req := range []EncryptRequest{
			{OutputPath: out, RecipientKey: f.bob.pub},
			{InputPath: f.plain, RecipientKey: f.bob.pub},
			{InputPath: f.plain, OutputPath: out},
			{InputPath: f.plain, OutputPath: out, RecipientKey: f.bob.pub, Passphrase: []byte("x")},
		} {
			if _, err := Encrypt(context.Background(), req); !errors.As(err, &ve) {
				t.Fatalf("req %+v: err = %v, want ValidationError", req, err)
			}
		}
	})

	t.Run("garbage key", func(t *testing.T) {
		bad := filepath.Join(f.dir, "bad.asc")
		writeFile(t, bad, []byte("not a key"))
		var ke *KeyError
		if _, err := Encrypt(context.Background(), EncryptRequest{InputPath: f.plain, OutputPath: out, RecipientKey: bad}); !errors.As(err, &ke) {
			t.Fatalf("err = %v, want KeyError", err)
		}
	})
}

func TestDecryptErrors(t *testing.T) {
	f := setup(t)
	enc := f.encryptToBob(t, true, true)
	out := filepath.Join(f.dir, "out.txt")

	carol := newKeyPair(t, f.dir, "carol", nil)
	cases := []struct {
		name string
		req  DecryptRequest
		want error
	}{
		{"not recipient", DecryptRequest{InputPath: enc, OutputPath: out, PrivateKey: f.alice.priv, Passphrase: f.alicePass}, ErrNoMatchingKey},
		{"wrong signer", DecryptRequest{InputPath: enc, OutputPath: out, PrivateKey: f.bob.priv, VerifyKey: carol.pub}, ErrSignerMismatch},
		{"public key as private", DecryptRequest{InputPath: enc, OutputPath: out, PrivateKey: f.bob.pub}, ErrNotPrivateKey},
		{"plaintext input", DecryptRequest{InputPath: f.plain, OutputPath: out, PrivateKey: f.bob.priv}, ErrNotEncrypted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decrypt(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if _, statErr := os.Stat(out); statErr == nil {
				t.Fatal("output must not exist after failure")
			}
		})
	}

	t.Run("unsigned but verify requested", func(t *testing.T) {
		unsigned := f.encryptToBob(t, false, false)
		_, err := Decrypt(context.Background(), DecryptRequest{InputPath: unsigned, OutputPath: out, PrivateKey: f.bob.priv, VerifyKey: f.alice.pub})
		if !errors.Is(err, ErrNotSigned) {
			t.Fatalf("err = %v, want ErrNotSigned", err)
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		bin := f.encryptToBob(t, false, false)
		data, _ := os.ReadFile(bin)
		data[len(data)-40] ^= 0xFF
		tampered := filepath.Join(f.dir, "tampered.gpg")
		writeFile(t, tampered, data)
		_, err := Decrypt(context.Background(), DecryptRequest{InputPath: tampered, OutputPath: out, PrivateKey: f.bob.priv})
		if err == nil {
			t.Fatal("expected error for tampered ciphertext")
		}
		if _, statErr := os.Stat(out); statErr == nil {
			t.Fatal("tampered plaintext must not be written")
		}
	})

	t.Run("symmetric only", func(t *testing.T) {
		sym := filepath.Join(f.dir, "sym.gpg")
		fh, _ := os.Create(sym)
		w, err := openpgp.SymmetricallyEncrypt(fh, []byte("pw"), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("x"))
		w.Close()
		fh.Close()
		if _, err := Decrypt(context.Background(), DecryptRequest{InputPath: sym, OutputPath: out, PrivateKey: f.bob.priv}); !errors.Is(err, ErrSymmetricOnly) {
			t.Fatalf("err = %v, want ErrSymmetricOnly", err)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		empty := filepath.Join(f.dir, "empty.gpg")
		writeFile(t, empty, nil)
		if _, err := Decrypt(context.Background(), DecryptRequest{InputPath: empty, OutputPath: out, PrivateKey: f.bob.priv}); !errors.Is(err, ErrNotEncrypted) {
			t.Fatalf("err = %v, want ErrNotEncrypted", err)
		}
	})
}

func TestCancel(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := filepath.Join(f.dir, "c.gpg")
	_, err := Encrypt(ctx, EncryptRequest{InputPath: f.plain, OutputPath: out, RecipientKey: f.bob.pub})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("canceled run left output behind")
	}
	entries, _ := os.ReadDir(f.dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestDefaultOutputs(t *testing.T) {
	sep := string(filepath.Separator)
	for in, want := range map[string]string{
		"a" + sep + "doc.pdf.gpg": "a" + sep + "doc.pdf",
		"doc.PGP":                 "doc",
		"doc.asc":                 "doc",
		"doc.bin":                 "doc.bin.decrypted",
		".gpg":                    ".gpg.decrypted",
		"":                        "",
	} {
		if got := DefaultDecryptOutput(in); got != want {
			t.Errorf("DefaultDecryptOutput(%q) = %q, want %q", in, got, want)
		}
	}
	if got := DefaultEncryptOutput("x", true); got != "x.asc" {
		t.Errorf("armored = %q", got)
	}
	if got := DefaultEncryptOutput("x", false); got != "x.gpg" {
		t.Errorf("binary = %q", got)
	}
}

// TestGnuPGInterop checks that GnuPG can decrypt our output. Skipped when gpg is absent.
func TestGnuPGInterop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("gpg path handling differs on Windows runners")
	}
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Skip("gpg not installed")
	}
	f := setup(t)
	home := t.TempDir()
	run := func(args ...string) []byte {
		cmd := exec.Command(gpg, append([]string{"--homedir", home, "--batch", "--yes", "--pinentry-mode", "loopback"}, args...)...)
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("gpg %v: %v\n%s", args, err, b)
		}
		return b
	}
	run("--import", f.bob.priv)
	enc := f.encryptToBob(t, true, false)
	dec := filepath.Join(f.dir, "gpg-out.txt")
	run("--output", dec, "--decrypt", enc)
	got, _ := os.ReadFile(dec)
	if !bytes.Equal(got, f.content) {
		t.Fatal("gpg plaintext mismatch")
	}

	run("--import", f.alice.pub)
	gpgEnc := filepath.Join(f.dir, "from-gpg.gpg")
	run("--trust-model", "always", "--output", gpgEnc, "--recipient", "bob@example.test", "--encrypt", f.plain)
	out := filepath.Join(f.dir, "ours.txt")
	if _, err := Decrypt(context.Background(), DecryptRequest{InputPath: gpgEnc, OutputPath: out, PrivateKey: f.bob.priv}); err != nil {
		t.Fatalf("decrypt gpg output: %v", err)
	}
	got, _ = os.ReadFile(out)
	if !bytes.Equal(got, f.content) {
		t.Fatal("plaintext mismatch for gpg-produced message")
	}
}
