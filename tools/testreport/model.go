package main

import (
	"sort"
	"time"
)

type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

type Test struct {
	Name    string
	Status  Status
	Elapsed float64
	Output  []string
}

type Package struct {
	Name        string
	Status      Status
	Elapsed     float64
	Tests       []*Test
	Output      []string // package-level output (not attributed to a test)
	BuildOutput []string
	BuildFailed bool
	Coverage    *float64 // percent of statements; nil when no profile data

	tests map[string]*Test
}

// NoTests reports a package that compiled but has no test functions.
func (p *Package) NoTests() bool {
	return len(p.Tests) == 0 && !p.BuildFailed && p.Status != StatusFail
}

type Report struct {
	Packages  []*Package
	Generated time.Time
	// Stray holds non-JSON input lines, e.g. a toolchain error printed before test2json started.
	Stray    []string
	Coverage *float64
}

type Totals struct {
	Packages, FailedPackages       int
	Tests, Passed, Failed, Skipped int
	Elapsed                        float64
}

func (r *Report) Totals() Totals {
	var t Totals
	for _, p := range r.Packages {
		t.Packages++
		if p.Status == StatusFail {
			t.FailedPackages++
		}
		t.Elapsed += p.Elapsed
		for _, tc := range p.Tests {
			t.Tests++
			switch tc.Status {
			case StatusPass:
				t.Passed++
			case StatusFail:
				t.Failed++
			case StatusSkip:
				t.Skipped++
			}
		}
	}
	return t
}

// Failed is true when any package or test failed.
func (r *Report) Failed() bool {
	t := r.Totals()
	return t.FailedPackages > 0 || t.Failed > 0
}

func (r *Report) FailedTests() []FailedTest {
	var out []FailedTest
	for _, p := range r.Packages {
		for _, t := range p.Tests {
			if t.Status == StatusFail {
				out = append(out, FailedTest{Package: p.Name, Test: t})
			}
		}
	}
	return out
}

type FailedTest struct {
	Package string
	Test    *Test
}

func (r *Report) sortPackages() {
	sort.SliceStable(r.Packages, func(i, j int) bool { return r.Packages[i].Name < r.Packages[j].Name })
}
