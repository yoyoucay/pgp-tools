// Package ui builds the Fyne desktop interface.
package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"

	"github.com/yoyoucay/pgp-tools/internal/formcheck"
	"github.com/yoyoucay/pgp-tools/internal/install"
	"github.com/yoyoucay/pgp-tools/internal/pgp"
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
func Run(version string, assets install.Assets) {
	a := app.NewWithID(AppID)
	res := fyne.NewStaticResource("pgp-tools.svg", assets.IconSVG)
	a.SetIcon(res)

	win := a.NewWindow("PGP Tools")
	win.SetIcon(res)

	checker := formcheck.NewChecker()
	enc := newEncryptTab(a, win, checker)
	dec := newDecryptTab(a, win, checker)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Encrypt", theme.VisibilityOffIcon(), enc.content),
		container.NewTabItemWithIcon("Decrypt", theme.VisibilityIcon(), dec.content),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		for _, u := range uris {
			path := filepath.FromSlash(u.Path())
			current := formcheck.Tab(tabs.SelectedIndex())
			tab, field := formcheck.Route(pgp.Classify(path), current)
			tabs.SelectIndex(int(tab))
			if tab == formcheck.TabEncrypt {
				enc.setField(field, path)
			} else {
				dec.setField(field, path)
			}
		}
	})

	tools := fyne.NewMenu("Tools",
		fyne.NewMenuItem("Encrypt", func() { tabs.SelectIndex(int(formcheck.TabEncrypt)) }),
		fyne.NewMenuItem("Decrypt", func() { tabs.SelectIndex(int(formcheck.TabDecrypt)) }),
	)
	if install.Supported() {
		tools.Items = append(tools.Items,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(installLabel(), func() { confirmInstall(win, assets) }),
			fyne.NewMenuItem("Remove from this computer...", func() { confirmUninstall(win) }),
		)
	}
	win.SetMainMenu(fyne.NewMainMenu(
		tools,
		fyne.NewMenu("Help",
			fyne.NewMenuItem("Which key goes where?", func() { dialog.ShowInformation("Which key goes where?", keyRolesText, win) }),
			fyne.NewMenuItem("About", func() {
				dialog.ShowInformation("About", "PGP Tools "+version+"\nOpenPGP file encryption. No GnuPG needed. No network access.", win)
			}),
		),
	))

	win.SetContent(tabs)
	win.Resize(fyne.NewSize(800, 640))
	win.CenterOnScreen()
	win.ShowAndRun()
}

func installLabel() string {
	if runtime.GOOS == "windows" {
		return "Install to Start Menu..."
	}
	return "Install to app menu..."
}

func confirmInstall(win fyne.Window, assets install.Assets) {
	p, err := install.UserPaths()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		dialog.ShowError(fmt.Errorf("cannot locate this program: %w", err), win)
		return
	}
	msg := fmt.Sprintf("Copy PGP Tools for your user account only. No administrator rights needed.\n\nProgram: %s\nMenu entry: %s", p.Binary(), p.MenuEntry())
	dialog.ShowConfirm("Install PGP Tools", msg, func(okPressed bool) {
		if !okPressed {
			return
		}
		go func() {
			err := install.Install(p, exe, assets)
			fyne.Do(func() {
				switch {
				case errors.Is(err, install.ErrMenuEntry):
					dialog.ShowInformation("Installed without menu entry",
						"The program was copied to:\n"+p.Binary()+"\n\nThe menu entry could not be created:\n"+strings.TrimPrefix(err.Error(), install.ErrMenuEntry.Error()+": ")+"\n\nYou can pin the program manually.", win)
				case err != nil:
					dialog.ShowError(err, win)
				default:
					dialog.ShowInformation("Installed", "PGP Tools is now in your "+menuName()+".\nYou can delete the downloaded copy.", win)
				}
			})
		}()
	}, win)
}

func confirmUninstall(win fyne.Window) {
	p, err := install.UserPaths()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	dialog.ShowConfirm("Remove PGP Tools", "Remove the installed program and its "+menuName()+" entry?\nYour keys and encrypted files are not touched.", func(okPressed bool) {
		if !okPressed {
			return
		}
		if err := install.Uninstall(p); err != nil {
			dialog.ShowError(err, win)
			return
		}
		dialog.ShowInformation("Removed", "PGP Tools was removed from your "+menuName()+".", win)
	}, win)
}

func menuName() string {
	if runtime.GOOS == "windows" {
		return "Start Menu"
	}
	return "app menu"
}

const keyRolesText = `Encrypt (protect a file for someone)
  Public key: required. The recipient's public key.
  Private key + passphrase: optional. Signs the file as you.

Decrypt (open a file sent to you)
  Private key + passphrase: required. Your own key.
  Public key: optional. The sender's key, to check the signature.

Never share your private key or passphrase.`
