package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/formcheck"
	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

const checkDelay = 250 * time.Millisecond

type encryptTab struct {
	input, pubKey, privKey, output *pathField
	pass                           *passField
	armor                          *widget.Check
	runner                         *runner
	live                           *liveCheck[formcheck.EncryptForm, formcheck.EncryptState]
	content                        fyne.CanvasObject
	outputEdited                   bool
}

func newEncryptTab(a fyne.App, win fyne.Window, checker *formcheck.Checker) *encryptTab {
	prefs := a.Preferences()
	t := &encryptTab{
		input:   newOpenField(win, "File to encrypt"),
		pubKey:  newOpenField(win, "Recipient's public key"),
		privKey: newOpenField(win, "Optional: your private key"),
		pass:    newPassField(),
		output:  newSaveField(win, "Where to save the encrypted file"),
		armor:   widget.NewCheck("Save as text (.asc), e.g. to paste into an email", nil),
	}
	t.live = &liveCheck[formcheck.EncryptForm, formcheck.EncryptState]{
		delay: checkDelay, gather: t.form, check: checker.Encrypt, apply: t.apply,
	}

	t.pubKey.SetText(prefs.String(prefEncPub))
	t.privKey.SetText(prefs.String(prefEncPriv))
	t.armor.SetChecked(prefs.Bool(prefEncArmor))

	t.input.entry.OnChanged = func(string) { t.syncOutput(); t.live.schedule() }
	t.armor.OnChanged = func(bool) { t.syncOutput(); t.live.schedule() }
	t.output.entry.OnChanged = func(string) {
		t.outputEdited = t.output.Text() != "" && t.output.Text() != t.defaultOutput()
		t.live.schedule()
	}
	t.pubKey.entry.OnChanged = func(string) { t.live.schedule() }
	t.privKey.entry.OnChanged = func(string) { t.live.schedule() }
	t.pass.entry.OnChanged = func(s string) {
		if s != "" {
			t.pass.status.set(formcheck.Result{Text: "Checking passphrase..."})
		}
		t.live.schedule()
	}

	t.runner = newRunner(win, "Encrypt", func() (string, job, error) {
		req := pgp.EncryptRequest{
			InputPath:    t.input.Text(),
			OutputPath:   t.output.Text(),
			RecipientKey: t.pubKey.Text(),
			SignerKey:    t.privKey.Text(),
			Armor:        t.armor.Checked,
		}
		if t.pass.allowed {
			req.Passphrase = []byte(t.pass.Text())
		}
		if err := req.Validate(); err != nil {
			return "", nil, err
		}
		prefs.SetString(prefEncPub, req.RecipientKey)
		prefs.SetString(prefEncPriv, req.SignerKey)
		prefs.SetBool(prefEncArmor, req.Armor)

		return req.OutputPath, func(ctx context.Context, overwrite bool) (string, error) {
			req.Overwrite = overwrite
			res, err := pgp.Encrypt(ctx, req)
			if err != nil {
				return "", err
			}
			fyne.Do(t.pass.Clear)
			signed := "not signed"
			if req.SignerKey != "" {
				signed = "signed by you"
			}
			return fmt.Sprintf("Encrypted %s (%s). Saved to:\n%s", formcheck.HumanBytes(res.Bytes), signed, req.OutputPath), nil
		}, nil
	})
	t.runner.inputs = []disabler{t.input, t.pubKey, t.privKey, t.pass, t.output, t.armor}
	t.runner.onDone = t.live.schedule

	form := widget.NewForm(
		widget.NewFormItem("Input file", t.input.Object()),
		widget.NewFormItem("Public key", t.pubKey.Object()),
		widget.NewFormItem("Private key", t.privKey.Object()),
		widget.NewFormItem("Passphrase", t.pass.box),
		widget.NewFormItem("Output file", t.output.Object()),
		widget.NewFormItem("", t.armor),
	)
	hint := widget.NewLabel("Protect a file so only the owner of the public key can open it. Drag files onto this window or use Browse.")
	hint.Wrapping = fyne.TextWrapWord
	t.content = container.NewVScroll(container.NewPadded(container.NewVBox(hint, form, t.runner.Object())))
	t.syncOutput()
	t.live.schedule()
	return t
}

func (t *encryptTab) defaultOutput() string {
	return pgp.DefaultEncryptOutput(t.input.Text(), t.armor.Checked)
}

func (t *encryptTab) syncOutput() {
	if !t.outputEdited {
		t.output.SetText(t.defaultOutput())
	}
}

func (t *encryptTab) form() formcheck.EncryptForm {
	return formcheck.EncryptForm{
		Input: t.input.Text(), PublicKey: t.pubKey.Text(), PrivateKey: t.privKey.Text(),
		Passphrase: t.pass.Text(), Output: t.output.Text(),
	}
}

func (t *encryptTab) apply(s formcheck.EncryptState) {
	t.input.status.set(s.Input)
	t.pubKey.status.set(s.PublicKey)
	t.privKey.status.set(s.PrivateKey)
	t.pass.status.set(s.Passphrase)
	t.output.status.set(s.Output)
	if !t.runner.busy {
		t.pass.setAllowed(s.PassphraseEnabled)
	} else {
		t.pass.allowed = s.PassphraseEnabled
	}
	t.runner.setReady(s.Ready, s.Summary)
}

func (t *encryptTab) setField(f formcheck.Field, path string) {
	switch f {
	case formcheck.FieldPublicKey:
		t.pubKey.SetText(path)
	case formcheck.FieldPrivateKey:
		t.privKey.SetText(path)
	default:
		t.input.SetText(path)
	}
}
