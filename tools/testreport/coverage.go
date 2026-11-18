package main

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

type blockCov struct {
	stmts   int
	covered bool
}

// ApplyCoverage reads a Go cover profile and sets per-package and total statement coverage.
// Blocks repeated across test binaries are merged: covered if any run hit them.
func ApplyCoverage(rep *Report, r io.Reader) error {
	blocks := map[string]*blockCov{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || (n == 1 && strings.HasPrefix(line, "mode:")) {
			continue
		}
		// file.go:12.34,15.2 numStmts count
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.Contains(fields[0], ":") {
			return fmt.Errorf("coverage line %d: malformed %q", n, line)
		}
		stmts, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("coverage line %d: malformed %q", n, line)
		}
		b, ok := blocks[fields[0]]
		if !ok {
			b = &blockCov{stmts: stmts}
			blocks[fields[0]] = b
		}
		b.covered = b.covered || count > 0
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read coverage: %w", err)
	}

	type agg struct{ total, covered int }
	perPkg := map[string]*agg{}
	var all agg
	for key, b := range blocks {
		file := key[:strings.LastIndex(key, ":")]
		pkg := path.Dir(file)
		a, ok := perPkg[pkg]
		if !ok {
			a = &agg{}
			perPkg[pkg] = a
		}
		a.total += b.stmts
		all.total += b.stmts
		if b.covered {
			a.covered += b.stmts
			all.covered += b.stmts
		}
	}
	for _, p := range rep.Packages {
		if a, ok := perPkg[p.Name]; ok && a.total > 0 {
			p.Coverage = percent(a.covered, a.total)
		}
	}
	if all.total > 0 {
		rep.Coverage = percent(all.covered, all.total)
	}
	return nil
}

func percent(n, d int) *float64 {
	v := float64(n) * 100 / float64(d)
	return &v
}
