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

// runner owns the run/cancel buttons, readiness summary, busy indicator and result
// line for one tab. The run button is enabled only when validation reports ready.
type runner struct {
	win      fyne.Window
	runBtn   *widget.Button
	cancel   *widget.Button
	summary  *widget.Label
	progress *widget.ProgressBarInfinite
	result   *widget.Label
	inputs   []disabler
	stop     context.CancelFunc
	ready    bool
	busy     bool
	onDone   func()
}

type job func(ctx context.Context, overwrite bool) (string, error)

func newRunner(win fyne.Window, label string, prepare func() (output string, j job, err error)) *runner {
	r := &runner{
		win:      win,
		summary:  widget.NewLabel(""),
		progress: widget.NewProgressBarInfinite(),
		result:   widget.NewLabel(""),
	}
	r.summary.Wrapping = fyne.TextWrapWord
	r.summary.TextStyle = fyne.TextStyle{Bold: true}
	r.result.Wrapping = fyne.TextWrapWord
	r.progress.Stop()
	r.progress.Hide()
	r.runBtn = widget.NewButtonWithIcon(label, theme.ConfirmIcon(), func() {
		out, j, err := prepare()
		if err != nil {
			r.fail(err)
			return
		}
		if _, statErr := os.Stat(out); statErr == nil {
			dialog.ShowConfirm("Replace file?", "This file already exists:\n"+out+"\n\nReplace it?", func(ok bool) {
				if ok {
					r.start(j, true)
				}
			}, win)
			return
		}
		r.start(j, false)
	})
	r.runBtn.Importance = widget.HighImportance
	r.runBtn.Disable()
	r.cancel = widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if r.stop != nil {
			r.stop()
			r.result.SetText("Canceling...")
		}
	})
	r.cancel.Disable()
	return r
}

func (r *runner) Object() fyne.CanvasObject {
	return container.NewVBox(
		r.summary,
		container.NewHBox(r.runBtn, r.cancel),
		r.progress,
		r.result,
	)
}

// setReady applies the latest validation verdict.
func (r *runner) setReady(ready bool, text string) {
	r.ready = ready
	r.summary.Importance = widget.MediumImportance
	if ready {
		r.summary.Importance = widget.SuccessImportance
	}
	r.summary.SetText(text)
	if !r.busy {
		r.syncRunButton()
	}
}

func (r *runner) syncRunButton() {
	if r.ready {
		r.runBtn.Enable()
	} else {
		r.runBtn.Disable()
	}
}

func (r *runner) start(j job, overwrite bool) {
	ctx, cancel := context.WithCancel(context.Background())
	r.stop = cancel
	r.setBusy(true)
	r.result.Importance = widget.MediumImportance
	r.result.SetText("Working...")
	go func() {
		defer cancel()
		msg, err := j(ctx, overwrite)
		fyne.Do(func() {
			r.stop = nil
			r.setBusy(false)
			if err != nil {
				r.fail(err)
			} else {
				r.result.Importance = widget.SuccessImportance
				r.result.SetText(msg)
				dialog.ShowInformation("Done", msg, r.win)
			}
			if r.onDone != nil {
				r.onDone()
			}
		})
	}()
}

func (r *runner) fail(err error) {
	text := describe(err)
	r.result.Importance = widget.DangerImportance
	r.result.SetText("Failed: " + text)
	if errors.Is(err, pgp.ErrCanceled) {
		r.result.Importance = widget.MediumImportance
		r.result.SetText("Canceled. No file was written.")
		return
	}
	dialog.ShowError(errors.New(text), r.win)
}

func (r *runner) setBusy(busy bool) {
	r.busy = busy
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
	r.syncRunButton()
	r.cancel.Disable()
	r.progress.Stop()
	r.progress.Hide()
}
