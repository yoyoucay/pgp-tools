// Command testreport converts `go test -json` output into JUnit XML, a Markdown
// summary, and a standalone HTML report. It uses only the standard library.
//
//	go test -json -coverprofile=reports/coverage.out ./... > reports/test.json
//	go run ./tools/testreport -in reports/test.json -coverprofile reports/coverage.out \
//	    -junit reports/junit.xml -md reports/summary.md -html reports/index.html
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	exitOK          = 0
	exitTestsFailed = 1
	exitUsage       = 2
	exitIO          = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("testreport", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		in       = fs.String("in", "-", "go test -json output file ('-' = stdin)")
		cover    = fs.String("coverprofile", "", "optional Go coverage profile")
		junit    = fs.String("junit", "", "write JUnit XML to this path")
		md       = fs.String("md", "", "write Markdown summary to this path ('-' = stdout)")
		html     = fs.String("html", "", "write HTML report to this path")
		title    = fs.String("title", "Unit test report", "HTML report title")
		covLink  = fs.String("coverage-link", "", "relative link from the HTML report to a coverage HTML page")
		failExit = fs.Bool("fail", false, "exit 1 when any test or package failed")
	)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *junit == "" && *md == "" && *html == "" {
		fmt.Fprintln(stderr, "testreport: nothing to write; set at least one of -junit, -md, -html")
		return exitUsage
	}

	rep, err := load(*in, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "testreport:", err)
		if errors.Is(err, ErrNoEvents) {
			return exitUsage
		}
		return exitIO
	}
	if *cover != "" {
		if err := applyCoverageFile(rep, *cover); err != nil {
			fmt.Fprintln(stderr, "testreport:", err)
			return exitIO
		}
	}

	outputs := []struct {
		path  string
		write func(io.Writer) error
	}{
		{*junit, func(w io.Writer) error { return WriteJUnit(w, rep) }},
		{*md, func(w io.Writer) error { return WriteMarkdown(w, rep) }},
		{*html, func(w io.Writer) error { return WriteHTML(w, rep, *title, *covLink) }},
	}
	for _, o := range outputs {
		if o.path == "" {
			continue
		}
		if err := writeOutput(o.path, stdout, o.write); err != nil {
			fmt.Fprintln(stderr, "testreport:", err)
			return exitIO
		}
	}

	t := rep.Totals()
	fmt.Fprintf(stderr, "testreport: %d tests, %d passed, %d failed, %d skipped, coverage %s\n",
		t.Tests, t.Passed, t.Failed, t.Skipped, pct(rep.Coverage))
	if *failExit && rep.Failed() {
		return exitTestsFailed
	}
	return exitOK
}

func load(path string, stdin io.Reader) (*Report, error) {
	if path == "-" {
		return Parse(stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()
	rep, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rep, nil
}

func applyCoverageFile(rep *Report, path string) error {
	f, err := os.Open(path)
	if err != nil {
		// A missing profile is expected when the build failed before tests ran.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open coverage: %w", err)
	}
	defer f.Close()
	return ApplyCoverage(rep, f)
}

// writeOutput renders fully in memory first so a render error never leaves a truncated file.
func writeOutput(path string, stdout io.Writer, write func(io.Writer) error) error {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		return err
	}
	if path == "-" {
		_, err := stdout.Write(buf.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
