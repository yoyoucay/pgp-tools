package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/formcheck"
	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

type decryptTab struct {
	input, privKey, pubKey, output *pathField
	pass                           *passField
	runner                         *runner
	live                           *liveCheck[formcheck.DecryptForm, formcheck.DecryptState]
	content                        fyne.CanvasObject
	outputEdited                   bool
}

func newDecryptTab(a fyne.App, win fyne.Window, checker *formcheck.Checker) *decryptTab {
	prefs := a.Preferences()
	t := &decryptTab{
		input:   newOpenField(win, "Encrypted file (.gpg, .pgp, .asc)"),
		privKey: newOpenField(win, "Your private key"),
		pass:    newPassField(),
		pubKey:  newOpenField(win, "Optional: sender's public key"),
		output:  newSaveField(win, "Where to save the decrypted file"),
	}
	t.live = &liveCheck[formcheck.DecryptForm, formcheck.DecryptState]{
		delay: checkDelay, gather: t.form, check: checker.Decrypt, apply: t.apply,
	}

	t.privKey.SetText(prefs.String(prefDecPriv))
	t.pubKey.SetText(prefs.String(prefDecPub))

	t.input.entry.OnChanged = func(string) {
		if !t.outputEdited {
			t.output.SetText(pgp.DefaultDecryptOutput(t.input.Text()))
		}
		t.live.schedule()
	}
	t.output.entry.OnChanged = func(string) {
		t.outputEdited = t.output.Text() != "" && t.output.Text() != pgp.DefaultDecryptOutput(t.input.Text())
		t.live.schedule()
	}
	t.privKey.entry.OnChanged = func(string) { t.live.schedule() }
	t.pubKey.entry.OnChanged = func(string) { t.live.schedule() }
	t.pass.entry.OnChanged = func(s string) {
		if s != "" {
			t.pass.status.set(formcheck.Result{Text: "Checking passphrase..."})
		}
		t.live.schedule()
	}

	t.runner = newRunner(win, "Decrypt", func() (string, job, error) {
		req := pgp.DecryptRequest{
			InputPath:  t.input.Text(),
			OutputPath: t.output.Text(),
			PrivateKey: t.privKey.Text(),
			VerifyKey:  t.pubKey.Text(),
		}
		if t.pass.allowed {
			req.Passphrase = []byte(t.pass.Text())
		}
		if err := req.Validate(); err != nil {
			return "", nil, err
		}
		prefs.SetString(prefDecPriv, req.PrivateKey)
		prefs.SetString(prefDecPub, req.VerifyKey)

		return req.OutputPath, func(ctx context.Context, overwrite bool) (string, error) {
			req.Overwrite = overwrite
			res, err := pgp.Decrypt(ctx, req)
			if err != nil {
				return "", err
			}
			fyne.Do(t.pass.Clear)
			return fmt.Sprintf("Decrypted %s. Saved to:\n%s\n%s", formcheck.HumanBytes(res.Bytes), req.OutputPath, signatureText(res)), nil
		}, nil
	})
	t.runner.inputs = []disabler{t.input, t.privKey, t.pass, t.pubKey, t.output}
	t.runner.onDone = t.live.schedule

	form := widget.NewForm(
		widget.NewFormItem("Input file", t.input.Object()),
		widget.NewFormItem("Private key", t.privKey.Object()),
		widget.NewFormItem("Passphrase", t.pass.box),
		widget.NewFormItem("Public key", t.pubKey.Object()),
		widget.NewFormItem("Output file", t.output.Object()),
	)
	hint := widget.NewLabel("Open a file that was encrypted for you. Drag files onto this window or use Browse.")
	hint.Wrapping = fyne.TextWrapWord
	t.content = container.NewVScroll(container.NewPadded(container.NewVBox(hint, form, t.runner.Object())))
	t.output.SetText(pgp.DefaultDecryptOutput(t.input.Text()))
	t.live.schedule()
	return t
}

func (t *decryptTab) form() formcheck.DecryptForm {
	return formcheck.DecryptForm{
		Input: t.input.Text(), PrivateKey: t.privKey.Text(), Passphrase: t.pass.Text(),
		PublicKey: t.pubKey.Text(), Output: t.output.Text(),
	}
}

func (t *decryptTab) apply(s formcheck.DecryptState) {
	t.input.status.set(s.Input)
	t.privKey.status.set(s.PrivateKey)
	t.pass.status.set(s.Passphrase)
	t.pubKey.status.set(s.PublicKey)
	t.output.status.set(s.Output)
	if !t.runner.busy {
		t.pass.setAllowed(s.PassphraseEnabled)
	} else {
		t.pass.allowed = s.PassphraseEnabled
	}
	t.runner.setReady(s.Ready, s.Summary)
}

func (t *decryptTab) setField(f formcheck.Field, path string) {
	switch f {
	case formcheck.FieldPublicKey:
		t.pubKey.SetText(path)
	case formcheck.FieldPrivateKey:
		t.privKey.SetText(path)
	default:
		t.input.SetText(path)
	}
}
