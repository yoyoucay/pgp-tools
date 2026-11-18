package main

import (
	"fmt"
	"io"
	"strings"
)

// maxFailureLines bounds each failure block so the summary stays within
// GitHub's 1 MiB job-summary limit even when a test floods output.
const maxFailureLines = 60

const noOutputNote = "No output. A parent test fails when any of its subtests fails."

// WriteMarkdown writes a summary suitable for $GITHUB_STEP_SUMMARY or a PR comment.
func WriteMarkdown(w io.Writer, rep *Report) error {
	t := rep.Totals()
	var b strings.Builder

	verdict := "PASS"
	if rep.Failed() {
		verdict = "FAIL"
	}
	fmt.Fprintf(&b, "## Unit test report: %s\n\n", verdict)
	b.WriteString("| Tests | Passed | Failed | Skipped | Packages | Test time | Coverage |\n")
	b.WriteString("|---:|---:|---:|---:|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %.2fs | %s |\n\n",
		t.Tests, t.Passed, t.Failed, t.Skipped, t.Packages, t.Elapsed, pct(rep.Coverage))

	b.WriteString("| Package | Status | Tests | Failed | Skipped | Test time | Coverage |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|\n")
	for _, p := range rep.Packages {
		var failed, skipped int
		for _, tc := range p.Tests {
			switch tc.Status {
			case StatusFail:
				failed++
			case StatusSkip:
				skipped++
			}
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d | %.2fs | %s |\n",
			mdCell(p.Name), packageLabel(p), len(p.Tests), failed, skipped, p.Elapsed, pct(p.Coverage))
	}

	if failures := rep.FailedTests(); len(failures) > 0 {
		b.WriteString("\n### Failed tests\n")
		for _, f := range failures {
			fmt.Fprintf(&b, "\n**%s** `%s`\n\n", mdCell(f.Test.Name), mdCell(f.Package))
			if len(f.Test.Output) == 0 {
				b.WriteString("_" + noOutputNote + "_\n")
				continue
			}
			writeFence(&b, f.Test.Output)
		}
	}
	for _, p := range rep.Packages {
		if p.BuildFailed {
			fmt.Fprintf(&b, "\n### Build failed: `%s`\n\n", mdCell(p.Name))
			writeFence(&b, p.BuildOutput)
		}
	}
	if len(rep.Stray) > 0 {
		b.WriteString("\n### Non-JSON output\n\n")
		lines := make([]string, len(rep.Stray))
		for i, s := range rep.Stray {
			lines[i] = s + "\n"
		}
		writeFence(&b, lines)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeFence(b *strings.Builder, lines []string) {
	body := strings.Join(tail(lines, maxFailureLines), "")
	// Pick a fence longer than any backtick run inside the output.
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	fmt.Fprintf(b, "%s\n%s", fence, body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence + "\n")
}

func tail(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	out := []string{fmt.Sprintf("... %d earlier lines omitted ...\n", len(lines)-n)}
	return append(out, lines[len(lines)-n:]...)
}

func mdCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ", "`", "'").Replace(s)
}

func pct(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", *v)
}

func packageLabel(p *Package) string {
	switch {
	case p.BuildFailed:
		return "BUILD FAILED"
	case p.Status == StatusFail:
		return "FAIL"
	case p.NoTests():
		return "no tests"
	case p.Status == StatusSkip:
		return "SKIP"
	default:
		return "PASS"
	}
}
