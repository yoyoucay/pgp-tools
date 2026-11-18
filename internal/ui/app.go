// Package ui builds the Fyne desktop interface.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
)

const (
	AppID = "io.github.yoyoucay.pgptools"

	// Only key file paths are persisted; passphrases never are.
	prefEncPub   = "encrypt.publicKey"
	prefEncPriv  = "encrypt.privateKey"
	prefEncArmor = "encrypt.armor"
	prefDecPriv  = "decrypt.privateKey"
	prefDecPub   = "decrypt.publicKey"
)

// Run opens the main window and blocks until it closes.
func Run(version string, icon []byte) {
	a := app.NewWithID(AppID)
	res := fyne.NewStaticResource("pgp-tools.svg", icon)
	a.SetIcon(res)

	win := a.NewWindow("PGP Tools")
	win.SetIcon(res)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Encrypt", theme.VisibilityOffIcon(), newEncryptTab(a, win)),
		container.NewTabItemWithIcon("Decrypt", theme.VisibilityIcon(), newDecryptTab(a, win)),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	win.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("Tools",
			fyne.NewMenuItem("Encrypt", func() { tabs.SelectIndex(0) }),
			fyne.NewMenuItem("Decrypt", func() { tabs.SelectIndex(1) }),
		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("Key roles", func() { dialog.ShowInformation("Key roles", keyRolesText, win) }),
			fyne.NewMenuItem("About", func() {
				dialog.ShowInformation("About", "PGP Tools "+version+"\nOpenPGP file encryption, no GnuPG required.", win)
			}),
		),
	))

	win.SetContent(tabs)
	win.Resize(fyne.NewSize(760, 520))
	win.CenterOnScreen()
	win.ShowAndRun()
}

const keyRolesText = `Encrypt
  Public key (recipient)  required - who can read it
  Private key + passphrase  optional - signs as you

Decrypt
  Private key + passphrase  required - your key
  Public key (sender)  optional - verifies signature`
