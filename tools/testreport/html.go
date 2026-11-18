package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"
)

//go:embed report.html.tmpl
var htmlTemplate string

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"pct":         pct,
	"label":       packageLabel,
	"skipReason":  skipReason,
	"join":        func(lines []string) string { return strings.Join(lines, "") },
	"upper":       func(s Status) string { return strings.ToUpper(string(s)) },
	"width":       func(v *float64) string { return fmt.Sprintf("%.1f", *v) },
	"statusClass": statusClass,
	"noOutput":    func() string { return noOutputNote },
	"count": func(tests []*Test, status string) int {
		n := 0
		for _, t := range tests {
			if string(t.Status) == status {
				n++
			}
		}
		return n
	},
}).Parse(htmlTemplate))

type htmlData struct {
	Title        string
	Verdict      string
	Failed       bool
	Generated    string
	CoverageLink string
	Totals       Totals
	Coverage     *float64
	Failures     []FailedTest
	Packages     []*Package
	Stray        []string
}

// WriteHTML writes a self-contained report (inline CSS, no scripts).
// html/template escapes test output, so arbitrary output cannot inject markup.
func WriteHTML(w io.Writer, rep *Report, title, coverageLink string) error {
	gen := rep.Generated
	if gen.IsZero() {
		gen = time.Now()
	}
	verdict := "PASS"
	if rep.Failed() {
		verdict = "FAIL"
	}
	return htmlTmpl.Execute(w, htmlData{
		Title:        title,
		Verdict:      verdict,
		Failed:       rep.Failed(),
		Generated:    gen.UTC().Format("2006-01-02 15:04:05 UTC"),
		CoverageLink: coverageLink,
		Totals:       rep.Totals(),
		Coverage:     rep.Coverage,
		Failures:     rep.FailedTests(),
		Packages:     rep.Packages,
		Stray:        rep.Stray,
	})
}

func statusClass(p *Package) string {
	switch {
	case p.Status == StatusFail || p.BuildFailed:
		return "fail"
	case p.NoTests(), p.Status == StatusSkip:
		return "muted"
	default:
		return "pass"
	}
}
