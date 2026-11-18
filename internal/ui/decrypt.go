package ui

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

func newDecryptTab(a fyne.App, win fyne.Window) fyne.CanvasObject {
	prefs := a.Preferences()

	input := newOpenField(win, "Encrypted file (.gpg / .pgp / .asc)")
	privKey := newOpenField(win, "Your private key (.asc / .gpg)")
	pass := widget.NewPasswordEntry()
	pass.SetPlaceHolder("Passphrase for the private key")
	pubKey := newOpenField(win, "Optional: sender public key, to verify signature")
	output := newSaveField(win, "Decrypted output file")

	privKey.SetText(prefs.String(prefDecPriv))
	pubKey.SetText(prefs.String(prefDecPub))

	outputEdited := false
	input.entry.OnChanged = func(s string) {
		if !outputEdited {
			output.SetText(pgp.DefaultDecryptOutput(strings.TrimSpace(s)))
		}
	}
	output.entry.OnChanged = func(s string) {
		outputEdited = s != "" && s != pgp.DefaultDecryptOutput(strings.TrimSpace(input.Text()))
	}

	r := newRunner(win, "Decrypt", theme.ConfirmIcon(), func() (string, job, error) {
		req := pgp.DecryptRequest{
			InputPath:  strings.TrimSpace(input.Text()),
			OutputPath: strings.TrimSpace(output.Text()),
			PrivateKey: strings.TrimSpace(privKey.Text()),
			Passphrase: []byte(pass.Text),
			VerifyKey:  strings.TrimSpace(pubKey.Text()),
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
			fyne.Do(func() { pass.SetText("") })
			return fmt.Sprintf("Decrypted %s to:\n%s\n%s", humanBytes(res.Bytes), req.OutputPath, signatureText(res)), nil
		}, nil
	})
	r.inputs = []disabler{input, privKey, pass, pubKey, output}

	form := widget.NewForm(
		widget.NewFormItem("Input file *", input.Object()),
		widget.NewFormItem("Private key *", privKey.Object()),
		widget.NewFormItem("Passphrase", pass),
		widget.NewFormItem("Public key", pubKey.Object()),
		widget.NewFormItem("Output file *", output.Object()),
	)
	hint := widget.NewLabel("Decrypts with your PRIVATE key (+ passphrase if protected). Sender's public key is optional; when given, a valid signature from it is required.")
	hint.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewPadded(container.NewVBox(hint, form, r.Object())))
}
