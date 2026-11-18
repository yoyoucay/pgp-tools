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

func newEncryptTab(a fyne.App, win fyne.Window) fyne.CanvasObject {
	prefs := a.Preferences()

	input := newOpenField(win, "File to encrypt")
	pubKey := newOpenField(win, "Recipient public key (.asc / .gpg)")
	privKey := newOpenField(win, "Optional: your private key, to sign")
	pass := widget.NewPasswordEntry()
	pass.SetPlaceHolder("Passphrase for the private key")
	output := newSaveField(win, "Encrypted output file")
	armor := widget.NewCheck("ASCII armor (.asc text output)", nil)

	pubKey.SetText(prefs.String(prefEncPub))
	privKey.SetText(prefs.String(prefEncPriv))
	armor.SetChecked(prefs.Bool(prefEncArmor))

	outputEdited := false
	syncOutput := func() {
		if !outputEdited {
			output.SetText(pgp.DefaultEncryptOutput(strings.TrimSpace(input.Text()), armor.Checked))
		}
	}
	input.entry.OnChanged = func(string) { syncOutput() }
	armor.OnChanged = func(bool) { syncOutput() }
	output.entry.OnChanged = func(s string) {
		outputEdited = s != "" && s != pgp.DefaultEncryptOutput(strings.TrimSpace(input.Text()), armor.Checked)
	}

	r := newRunner(win, "Encrypt", theme.ConfirmIcon(), func() (string, job, error) {
		req := pgp.EncryptRequest{
			InputPath:    strings.TrimSpace(input.Text()),
			OutputPath:   strings.TrimSpace(output.Text()),
			RecipientKey: strings.TrimSpace(pubKey.Text()),
			SignerKey:    strings.TrimSpace(privKey.Text()),
			Passphrase:   []byte(pass.Text),
			Armor:        armor.Checked,
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
			fyne.Do(func() { pass.SetText("") })
			signed := "unsigned"
			if req.SignerKey != "" {
				signed = "signed"
			}
			return fmt.Sprintf("Encrypted %s (%s) to:\n%s", humanBytes(res.Bytes), signed, req.OutputPath), nil
		}, nil
	})
	r.inputs = []disabler{input, pubKey, privKey, pass, output, armor}

	form := widget.NewForm(
		widget.NewFormItem("Input file *", input.Object()),
		widget.NewFormItem("Public key *", pubKey.Object()),
		widget.NewFormItem("Private key", privKey.Object()),
		widget.NewFormItem("Passphrase", pass),
		widget.NewFormItem("Output file *", output.Object()),
		widget.NewFormItem("", armor),
	)
	hint := widget.NewLabel("Encrypts with the recipient's PUBLIC key. Private key + passphrase are optional and only add your signature.")
	hint.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewPadded(container.NewVBox(hint, form, r.Object())))
}
