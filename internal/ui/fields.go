package ui

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/formcheck"
)

// pathField is an editable path entry with a Browse button and a status line.
type pathField struct {
	entry  *widget.Entry
	button *widget.Button
	status *statusLine
	box    fyne.CanvasObject
}

// Text returns the normalized path (quotes, slashes and ~ handled).
func (f *pathField) Text() string              { return formcheck.NormalizePath(f.entry.Text) }
func (f *pathField) SetText(s string)          { f.entry.SetText(s) }
func (f *pathField) Disable()                  { f.entry.Disable(); f.button.Disable() }
func (f *pathField) Enable()                   { f.entry.Enable(); f.button.Enable() }
func (f *pathField) Object() fyne.CanvasObject { return f.box }

// newOpenField picks an existing file.
func newOpenField(win fyne.Window, placeholder string) *pathField {
	f := &pathField{entry: widget.NewEntry(), status: newStatusLine()}
	f.entry.SetPlaceHolder(placeholder)
	f.button = widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if rc == nil {
				return
			}
			_ = rc.Close()
			f.entry.SetText(filepath.FromSlash(rc.URI().Path()))
		}, win)
		setStartDir(d, f.Text())
		d.Resize(dialogSize(win))
		d.Show()
	})
	f.layout()
	return f
}

// newSaveField picks an output directory and keeps the file name. A folder picker is
// used instead of Fyne's save dialog because that dialog creates/truncates the file
// immediately, which would destroy an existing file before the user confirms.
func newSaveField(win fyne.Window, placeholder string) *pathField {
	f := &pathField{entry: widget.NewEntry(), status: newStatusLine()}
	f.entry.SetPlaceHolder(placeholder)
	f.button = widget.NewButtonWithIcon("Folder", theme.FolderIcon(), func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if lu == nil {
				return
			}
			name := filepath.Base(f.Text())
			if f.Text() == "" || name == "." || name == string(filepath.Separator) {
				name = "output"
			}
			f.entry.SetText(filepath.Join(filepath.FromSlash(lu.Path()), name))
		}, win)
		setStartDir(d, f.Text())
		d.Resize(dialogSize(win))
		d.Show()
	})
	f.layout()
	return f
}

func (f *pathField) layout() {
	f.box = container.NewVBox(container.NewBorder(nil, nil, nil, f.button, f.entry), f.status.Object())
}

// passField is a password entry with a status line.
type passField struct {
	entry  *widget.Entry
	status *statusLine
	box    fyne.CanvasObject
	// allowed tracks validation's verdict so the busy-state toggle does not re-enable it.
	allowed bool
}

func newPassField() *passField {
	f := &passField{entry: widget.NewPasswordEntry(), status: newStatusLine()}
	f.entry.SetPlaceHolder("Passphrase for the private key")
	f.box = container.NewVBox(f.entry, f.status.Object())
	f.entry.Disable()
	return f
}

func (f *passField) Text() string { return f.entry.Text }
func (f *passField) Clear()       { f.entry.SetText("") }
func (f *passField) Disable()     { f.entry.Disable() }
func (f *passField) Enable()      { f.setAllowed(f.allowed) }
func (f *passField) setAllowed(b bool) {
	f.allowed = b
	if b {
		f.entry.Enable()
	} else {
		f.entry.Disable()
	}
}

func setStartDir(d *dialog.FileDialog, current string) {
	if current == "" {
		return
	}
	dir := current
	if fi, err := os.Stat(current); err != nil || !fi.IsDir() {
		dir = filepath.Dir(current)
	}
	if lu, err := storage.ListerForURI(storage.NewFileURI(dir)); err == nil {
		d.SetLocation(lu)
	}
}

func dialogSize(win fyne.Window) fyne.Size {
	s := win.Canvas().Size()
	return fyne.NewSize(s.Width*0.9, s.Height*0.9)
}
