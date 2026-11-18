package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/yoyoucay/pgp-tools/internal/formcheck"
)

// statusLine shows one field's validation result under the field.
type statusLine struct {
	icon *widget.Icon
	text *widget.Label
	box  *fyne.Container
}

func newStatusLine() *statusLine {
	s := &statusLine{icon: widget.NewIcon(nil), text: widget.NewLabel("")}
	s.text.Wrapping = fyne.TextWrapWord
	s.text.SizeName = theme.SizeNameCaptionText
	s.icon.Hide()
	s.box = container.NewBorder(nil, nil, s.icon, nil, s.text)
	return s
}

func (s *statusLine) Object() fyne.CanvasObject { return s.box }

func (s *statusLine) set(r formcheck.Result) {
	switch r.Level {
	case formcheck.OK:
		s.icon.SetResource(theme.NewSuccessThemedResource(theme.ConfirmIcon()))
		s.text.Importance = widget.SuccessImportance
	case formcheck.Warn:
		s.icon.SetResource(theme.NewWarningThemedResource(theme.WarningIcon()))
		s.text.Importance = widget.WarningImportance
	case formcheck.Error:
		s.icon.SetResource(theme.NewErrorThemedResource(theme.ErrorIcon()))
		s.text.Importance = widget.DangerImportance
	default:
		s.icon.SetResource(nil)
		s.text.Importance = widget.MediumImportance
	}
	if r.Level == formcheck.Neutral {
		s.icon.Hide()
	} else {
		s.icon.Show()
	}
	s.text.SetText(r.Text)
}

// liveCheck runs a form check in the background after input settles.
// gather runs on the UI thread; check runs off it; apply runs on the UI thread
// only if no newer input arrived meanwhile.
type liveCheck[F, S any] struct {
	delay  time.Duration
	gather func() F
	check  func(F) S
	apply  func(S)

	mu    sync.Mutex
	timer *time.Timer
	gen   uint64
}

func (l *liveCheck[F, S]) schedule() {
	form := l.gather()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gen++
	gen := l.gen
	if l.timer != nil {
		l.timer.Stop()
	}
	l.timer = time.AfterFunc(l.delay, func() {
		state := l.check(form)
		fyne.Do(func() {
			l.mu.Lock()
			current := gen == l.gen
			l.mu.Unlock()
			if current {
				l.apply(state)
			}
		})
	})
}
