package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// event mirrors cmd/test2json output (Go 1.24 adds ImportPath/FailedBuild for build events).
type event struct {
	Time        time.Time
	Action      string
	Package     string
	Test        string
	Output      string
	Elapsed     float64
	ImportPath  string
	FailedBuild string
}

var ErrNoEvents = errors.New("no go test -json events found in input")

const noResultNote = "no result recorded: test binary crashed, timed out, or was killed"

// Parse reads a `go test -json` stream. Non-JSON lines are kept in Report.Stray.
func Parse(r io.Reader) (*Report, error) {
	rep := &Report{}
	byName := map[string]*Package{}
	pkg := func(name string) *Package {
		if p, ok := byName[name]; ok {
			return p
		}
		p := &Package{Name: name, tests: map[string]*Test{}}
		byName[name] = p
		rep.Packages = append(rep.Packages, p)
		return p
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	events := 0
	for line := 1; sc.Scan(); line++ {
		raw := sc.Bytes()
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		var ev event
		if raw[0] != '{' || json.Unmarshal(raw, &ev) != nil || ev.Action == "" {
			rep.Stray = append(rep.Stray, string(raw))
			continue
		}
		events++
		if ev.Time.After(rep.Generated) {
			rep.Generated = ev.Time
		}
		apply(ev, pkg)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if events == 0 {
		return nil, ErrNoEvents
	}

	for _, p := range rep.Packages {
		finalize(p)
	}
	rep.sortPackages()
	return rep, nil
}

func apply(ev event, pkg func(string) *Package) {
	switch ev.Action {
	case "build-output", "build-fail":
		p := pkg(buildPackage(ev.ImportPath))
		if ev.Action == "build-fail" {
			p.BuildFailed = true
			p.Status = StatusFail
		} else {
			p.BuildOutput = append(p.BuildOutput, ev.Output)
		}
		return
	}
	if ev.Package == "" {
		return
	}
	p := pkg(ev.Package)

	if ev.Test == "" {
		switch ev.Action {
		case "output":
			p.Output = append(p.Output, ev.Output)
		case "pass", "fail", "skip":
			p.Status = Status(ev.Action)
			p.Elapsed = ev.Elapsed
			if ev.FailedBuild != "" {
				p.BuildFailed = true
			}
		}
		return
	}

	t, ok := p.tests[ev.Test]
	if !ok {
		t = &Test{Name: ev.Test}
		p.tests[ev.Test] = t
		p.Tests = append(p.Tests, t)
	}
	switch ev.Action {
	case "output":
		if !isFrameworkLine(ev.Output) {
			t.Output = append(t.Output, ev.Output)
		}
	case "pass", "fail", "skip":
		t.Status = Status(ev.Action)
		t.Elapsed = ev.Elapsed
	}
}

func finalize(p *Package) {
	for _, t := range p.Tests {
		if t.Status == "" {
			t.Status = StatusFail
			t.Output = append(t.Output, noResultNote+"\n")
		}
	}
	if p.Status == "" {
		// Package started but never reported; treat as failure so it is not silently dropped.
		p.Status = StatusFail
		p.Output = append(p.Output, noResultNote+"\n")
	}
}

// buildPackage maps "pkg [pkg.test]" to "pkg".
func buildPackage(importPath string) string {
	if i := strings.Index(importPath, " ["); i >= 0 {
		return importPath[:i]
	}
	return importPath
}

func isFrameworkLine(s string) bool {
	trimmed := strings.TrimLeft(s, " ")
	for _, prefix := range []string{"=== RUN", "=== PAUSE", "=== CONT", "=== NAME", "--- PASS", "--- SKIP"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
