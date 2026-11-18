package ui

import (
	"context"
	"errors"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/pgp"
)

type disabler interface {
	Disable()
	Enable()
}

// runner owns the run/cancel buttons, busy indicator, and status line for one tab.
// Only one job runs per tab; inputs are locked while it runs.
type runner struct {
	win      fyne.Window
	runBtn   *widget.Button
	cancel   *widget.Button
	progress *widget.ProgressBarInfinite
	status   *widget.Label
	inputs   []disabler
	stop     context.CancelFunc
}

type job func(ctx context.Context, overwrite bool) (string, error)

func newRunner(win fyne.Window, label string, icon fyne.Resource, prepare func() (output string, j job, err error)) *runner {
	r := &runner{
		win:      win,
		progress: widget.NewProgressBarInfinite(),
		status:   widget.NewLabel("Ready."),
	}
	r.status.Wrapping = fyne.TextWrapWord
	r.progress.Stop()
	r.progress.Hide()
	r.runBtn = widget.NewButtonWithIcon(label, icon, func() {
		out, j, err := prepare()
		if err != nil {
			r.fail(err)
			return
		}
		if _, statErr := os.Stat(out); statErr == nil {
			dialog.ShowConfirm("Overwrite file?", "Output file already exists:\n"+out+"\n\nReplace it?", func(ok bool) {
				if ok {
					r.start(j, true)
				}
			}, win)
			return
		}
		r.start(j, false)
	})
	r.runBtn.Importance = widget.HighImportance
	r.cancel = widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if r.stop != nil {
			r.stop()
			r.status.SetText("Canceling...")
		}
	})
	r.cancel.Disable()
	return r
}

func (r *runner) Object() fyne.CanvasObject {
	return container.NewVBox(
		container.NewHBox(r.runBtn, r.cancel),
		r.progress,
		r.status,
	)
}

func (r *runner) start(j job, overwrite bool) {
	ctx, cancel := context.WithCancel(context.Background())
	r.stop = cancel
	r.setBusy(true)
	r.status.SetText("Working...")
	go func() {
		defer cancel()
		msg, err := j(ctx, overwrite)
		fyne.Do(func() {
			r.stop = nil
			r.setBusy(false)
			if err != nil {
				r.fail(err)
				return
			}
			r.status.SetText(msg)
			dialog.ShowInformation("Done", msg, r.win)
		})
	}()
}

func (r *runner) fail(err error) {
	text := describe(err)
	r.status.SetText("Failed: " + text)
	if errors.Is(err, pgp.ErrCanceled) {
		return
	}
	dialog.ShowError(errors.New(text), r.win)
}

func (r *runner) setBusy(busy bool) {
	for _, in := range r.inputs {
		if busy {
			in.Disable()
		} else {
			in.Enable()
		}
	}
	if busy {
		r.runBtn.Disable()
		r.cancel.Enable()
		r.progress.Show()
		r.progress.Start()
		return
	}
	r.runBtn.Enable()
	r.cancel.Disable()
	r.progress.Stop()
	r.progress.Hide()
}
