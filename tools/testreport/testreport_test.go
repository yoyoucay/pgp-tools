package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) *Report {
	t.Helper()
	f, err := os.Open("testdata/mixed.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rep, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	cov, err := os.Open("testdata/coverage.profile")
	if err != nil {
		t.Fatal(err)
	}
	defer cov.Close()
	if err := ApplyCoverage(rep, cov); err != nil {
		t.Fatal(err)
	}
	return rep
}

func pkgByName(t *testing.T, rep *Report, name string) *Package {
	t.Helper()
	for _, p := range rep.Packages {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("package %s missing", name)
	return nil
}

func TestParseTotals(t *testing.T) {
	rep := loadFixture(t)
	got := rep.Totals()
	want := Totals{Packages: 4, FailedPackages: 3, Tests: 5, Passed: 1, Failed: 3, Skipped: 1, Elapsed: 0.05}
	if got != want {
		t.Fatalf("totals = %+v, want %+v", got, want)
	}
	if !rep.Failed() {
		t.Fatal("report must be failed")
	}
	if len(rep.Stray) != 1 || !strings.Contains(rep.Stray[0], "stray toolchain line") {
		t.Fatalf("stray = %q", rep.Stray)
	}
}

func TestParsePackageStates(t *testing.T) {
	rep := loadFixture(t)

	broken := pkgByName(t, rep, "example.com/m/broken")
	if !broken.BuildFailed || len(broken.BuildOutput) != 2 {
		t.Fatalf("broken: buildFailed=%v output=%q", broken.BuildFailed, broken.BuildOutput)
	}

	crash := pkgByName(t, rep, "example.com/m/crash")
	if crash.Status != StatusFail || crash.Tests[0].Status != StatusFail {
		t.Fatal("test without terminal event must count as failed")
	}
	if last := crash.Tests[0].Output[len(crash.Tests[0].Output)-1]; !strings.Contains(last, "no result recorded") {
		t.Fatalf("missing crash note, got %q", last)
	}

	if empty := pkgByName(t, rep, "example.com/m/empty"); !empty.NoTests() {
		t.Fatal("empty package must report NoTests")
	}

	core := pkgByName(t, rep, "example.com/m/core")
	for _, tc := range core.Tests {
		for _, line := range tc.Output {
			if strings.HasPrefix(strings.TrimSpace(line), "=== RUN") || strings.HasPrefix(strings.TrimSpace(line), "--- PASS") {
				t.Fatalf("framework line kept in %s: %q", tc.Name, line)
			}
		}
	}
	if names := []string{rep.Packages[0].Name, rep.Packages[3].Name}; names[0] != "example.com/m/broken" || names[1] != "example.com/m/empty" {
		t.Fatalf("packages not sorted: %v", names)
	}
}

func TestParseRejectsNonTestInput(t *testing.T) {
	for _, in := range []string{"", "\n\n", "PASS\nok  \tpkg\t0.1s\n", `{"foo":1}`} {
		if _, err := Parse(strings.NewReader(in)); !errors.Is(err, ErrNoEvents) {
			t.Errorf("Parse(%q) err = %v, want ErrNoEvents", in, err)
		}
	}
}

func TestParseLongLine(t *testing.T) {
	long := strings.Repeat("x", 1<<20)
	in := `{"Action":"run","Package":"p","Test":"T"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"T","Output":"` + long + `\n"}` + "\n" +
		`{"Action":"pass","Package":"p","Test":"T"}` + "\n" +
		`{"Action":"pass","Package":"p"}` + "\n"
	rep, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("1 MiB output line: %v", err)
	}
	if rep.Totals().Passed != 1 {
		t.Fatal("expected one passing test")
	}
}

func TestCoverage(t *testing.T) {
	rep := loadFixture(t)
	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s coverage = %v, want %v", name, got, want)
		}
	}
	// core: 3 + 1 (duplicate block merged as covered) of 8 statements.
	check("core", pkgByName(t, rep, "example.com/m/core").Coverage, 50)
	check("empty", pkgByName(t, rep, "example.com/m/empty").Coverage, 0)
	check("total", rep.Coverage, 40)
	if pkgByName(t, rep, "example.com/m/broken").Coverage != nil {
		t.Error("package absent from profile must have nil coverage")
	}

	for _, bad := range []string{"mode: set\nnot a line\n", "mode: set\nf.go:1.1,2.2 x 1\n", "mode: set\nnocolon 1 1\n"} {
		if err := ApplyCoverage(&Report{}, strings.NewReader(bad)); err == nil {
			t.Errorf("ApplyCoverage(%q) accepted malformed input", bad)
		}
	}
}

func TestJUnit(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJUnit(&buf, loadFixture(t)); err != nil {
		t.Fatal(err)
	}
	var doc junitSuites
	if err := xml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	// 5 tests + [build] error for broken; crash already has a failed test so no extra error.
	if doc.Tests != 6 || doc.Failures != 3 || doc.Errors != 1 || doc.Skipped != 1 {
		t.Fatalf("junit totals tests=%d failures=%d errors=%d skipped=%d", doc.Tests, doc.Failures, doc.Errors, doc.Skipped)
	}
	for _, s := range doc.Suites {
		if s.Name != "example.com/m/core" {
			continue
		}
		for _, c := range s.Cases {
			if c.Name == "TestSkip" && (c.Skipped == nil || c.Skipped.Message != "core_test.go:20: gpg not installed") {
				t.Fatalf("skip reason = %+v", c.Skipped)
			}
		}
	}
}

func TestMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, loadFixture(t)); err != nil {
		t.Fatal(err)
	}
	md := buf.String()
	for _, want := range []string{
		"## Unit test report: FAIL",
		"| 5 | 1 | 3 | 1 | 4 |",
		"| BUILD FAILED |",
		"| no tests |",
		`TestBad/sub\|pipe`,
		"### Build failed: `example.com/m/broken`",
		"````\n", // fence lengthened because output contains ```
		"### Non-JSON output",
		"_" + noOutputNote + "_",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestMarkdownTruncatesFloodedOutput(t *testing.T) {
	lines := make([]string, maxFailureLines+40)
	for i := range lines {
		lines[i] = "line\n"
	}
	got := tail(lines, maxFailureLines)
	if len(got) != maxFailureLines+1 || !strings.Contains(got[0], "40 earlier lines omitted") {
		t.Fatalf("tail len=%d first=%q", len(got), got[0])
	}
}

func TestHTMLEscapesOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHTML(&buf, loadFixture(t), "Report", "coverage.html"); err != nil {
		t.Fatal(err)
	}
	h := buf.String()
	if strings.Contains(h, "<script>") {
		t.Fatal("test output was not escaped")
	}
	for _, want := range []string{"&lt;script&gt;", `href="coverage.html"`, "Build failed:", "no tests", "50.0%", noOutputNote} {
		if !strings.Contains(h, want) {
			t.Errorf("html missing %q", want)
		}
	}
}

func TestRunCLI(t *testing.T) {
	dir := t.TempDir()
	junit := filepath.Join(dir, "nested", "junit.xml")
	html := filepath.Join(dir, "index.html")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-in", "testdata/mixed.json", "-coverprofile", "testdata/coverage.profile",
		"-junit", junit, "-html", html, "-md", "-"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	for _, p := range []string{junit, html} {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Fatalf("%s not written: %v", p, err)
		}
	}
	if !strings.Contains(stdout.String(), "Unit test report") {
		t.Fatal("markdown not written to stdout")
	}

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"fail flag", []string{"-in", "testdata/mixed.json", "-md", filepath.Join(dir, "s.md"), "-fail"}, exitTestsFailed},
		{"no outputs", []string{"-in", "testdata/mixed.json"}, exitUsage},
		{"missing input", []string{"-in", filepath.Join(dir, "nope.json"), "-md", "-"}, exitIO},
		{"empty input", []string{"-in", os.DevNull, "-md", "-"}, exitUsage},
		{"bad flag", []string{"-nope"}, exitUsage},
		{"missing coverage tolerated", []string{"-in", "testdata/mixed.json", "-coverprofile", filepath.Join(dir, "none.out"), "-md", "-"}, exitOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); got != tc.want {
				t.Fatalf("exit = %d, want %d", got, tc.want)
			}
		})
	}
}
