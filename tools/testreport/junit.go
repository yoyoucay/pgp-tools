package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	Timestamp string          `xml:"timestamp,attr,omitempty"`
	Props     *junitProps     `xml:"properties,omitempty"`
	Cases     []junitCase     `xml:"testcase"`
	SystemOut *junitTextBlock `xml:"system-out,omitempty"`
}

type junitProps struct {
	Items []junitProp `xml:"property"`
}

type junitProp struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitCase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
}

type junitMessage struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type junitTextBlock struct {
	Body string `xml:",chardata"`
}

// WriteJUnit emits JUnit XML. Build failures and package failures without a failing
// test become an <error> testcase so CI tools do not report the package as green.
func WriteJUnit(w io.Writer, rep *Report) error {
	tot := rep.Totals()
	out := junitSuites{Name: "go test", Time: secs(tot.Elapsed)}
	for _, p := range rep.Packages {
		s := junitSuite{Name: p.Name, Time: secs(p.Elapsed)}
		if !rep.Generated.IsZero() {
			s.Timestamp = rep.Generated.UTC().Format("2006-01-02T15:04:05")
		}
		if p.Coverage != nil {
			s.Props = &junitProps{Items: []junitProp{{Name: "coverage.statements.pct", Value: fmt.Sprintf("%.1f", *p.Coverage)}}}
		}
		failedTests := 0
		for _, t := range p.Tests {
			c := junitCase{Classname: p.Name, Name: t.Name, Time: secs(t.Elapsed)}
			switch t.Status {
			case StatusFail:
				c.Failure = &junitMessage{Message: "Failed", Body: strings.Join(t.Output, "")}
				s.Failures++
				failedTests++
			case StatusSkip:
				c.Skipped = &junitMessage{Message: skipReason(t.Output)}
				s.Skipped++
			}
			s.Cases = append(s.Cases, c)
		}
		if p.BuildFailed || (p.Status == StatusFail && failedTests == 0) {
			name, body := "[package]", strings.Join(p.Output, "")
			if p.BuildFailed {
				name, body = "[build]", strings.Join(p.BuildOutput, "")+body
			}
			s.Cases = append(s.Cases, junitCase{Classname: p.Name, Name: name, Time: "0.000",
				Error: &junitMessage{Message: "Package failed", Body: body}})
			s.Errors++
		}
		s.Tests = len(s.Cases)
		if len(p.Output) > 0 {
			s.SystemOut = &junitTextBlock{Body: strings.Join(p.Output, "")}
		}
		out.Tests += s.Tests
		out.Failures += s.Failures
		out.Errors += s.Errors
		out.Skipped += s.Skipped
		out.Suites = append(out.Suites, s)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode junit: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func secs(f float64) string { return fmt.Sprintf("%.3f", f) }

// skipReason extracts the t.Skip message, which test2json reports as an indented output line.
func skipReason(output []string) string {
	for i := len(output) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(output[i]); s != "" {
			return s
		}
	}
	return "skipped"
}
