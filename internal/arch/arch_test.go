package arch_test

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/jobrunner/tempus"

// layerPrefixes maps a package path (relative to the module) to its layer.
// Order matters: the first matching prefix wins, so put specific before general.
var layerPrefixes = []struct{ prefix, layer string }{
	{"internal/domain", "domain"},
	{"internal/ports", "ports"},
	{"internal/application", "application"},
	{"internal/adapters", "adapters"},
	{"internal/config", "config"},
	{"internal/app", "app"},
	{"cmd", "cmd"},
}

// allowed[from] is the set of layers `from` may import. An empty set means "no
// internal imports at all". A layer missing from this map is a bug, not a
// licence: layerOf would have classified it, and the lookup then denies
// everything, which surfaces immediately as a failing test.
var allowed = map[string]map[string]bool{
	"domain":      {},
	"ports":       {"domain": true},
	"config":      {"domain": true},
	"application": {"domain": true, "ports": true},
	"adapters":    {"domain": true, "ports": true},
	"app":         {"domain": true, "ports": true, "application": true, "adapters": true, "config": true},
	"cmd":         {"domain": true, "ports": true, "application": true, "adapters": true, "config": true, "app": true},
}

// domainExternalAllowlist names non-stdlib imports the domain may use. Keep it
// empty if you possibly can — every entry is a dependency the business core
// drags into every test and every consumer.
var domainExternalAllowlist = map[string]bool{}

type goPkg struct {
	ImportPath string
	GoFiles    []string
	Imports    []string
}

// moduleRoot returns the module's root directory, so the test does not depend on
// how deep it happens to be nested.
func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("locating module root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// loadPackages shells out to `go list`. Deliberately NOT
// golang.org/x/tools/go/packages: that is a new go.mod dependency for
// information the toolchain already hands out for free.
func loadPackages(t *testing.T) []goPkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}

	var pkgs []goPkg
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var p goPkg
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	// Vacuity guard: a gate that silently measures nothing is worse than none.
	if len(pkgs) == 0 {
		t.Fatal("go list returned no packages — this gate would be vacuous")
	}
	return pkgs
}

// rel strips the module prefix. Returns "" for the module root and for anything
// outside the module.
func rel(importPath string) string {
	if importPath == modulePath {
		return ""
	}
	if !strings.HasPrefix(importPath, modulePath+"/") {
		return ""
	}
	return strings.TrimPrefix(importPath, modulePath+"/")
}

func layerOf(relPath string) (string, bool) {
	for _, lp := range layerPrefixes {
		if relPath == lp.prefix || strings.HasPrefix(relPath, lp.prefix+"/") {
			return lp.layer, true
		}
	}
	return "", false
}

// TestEveryPackageHasALayer closes hole 1: a package nobody classified is a
// package nobody guards.
func TestEveryPackageHasALayer(t *testing.T) {
	for _, p := range loadPackages(t) {
		r := rel(p.ImportPath)
		if r == "" || len(p.GoFiles) == 0 {
			continue // module root, or a test-only package such as this one
		}
		if _, ok := layerOf(r); !ok {
			t.Errorf("package %q maps to no layer.\n"+
				"Add a prefix to layerPrefixes (and a matching depguard rule), "+
				"or move the package under an existing layer.", r)
		}
	}
}

// TestImportsRespectLayerBoundaries closes hole 3: the rule is an allowlist, so
// a new layer cannot silently inherit permissions from anywhere.
func TestImportsRespectLayerBoundaries(t *testing.T) {
	baseline := loadBaseline(t)
	seen := map[string]bool{}

	for _, p := range loadPackages(t) {
		from := rel(p.ImportPath)
		if from == "" || len(p.GoFiles) == 0 {
			continue
		}
		fromLayer, ok := layerOf(from)
		if !ok {
			continue // already reported by TestEveryPackageHasALayer
		}
		for _, imp := range p.Imports {
			to := rel(imp)
			if to == "" {
				continue // stdlib or third-party, not an internal edge
			}
			toLayer, ok := layerOf(to)
			if !ok {
				continue
			}
			if fromLayer == toLayer {
				continue // intra-layer imports are fine
			}
			if allowed[fromLayer][toLayer] {
				continue
			}
			key := from + " -> " + to
			seen[key] = true
			if !baseline[key] {
				t.Errorf("forbidden import %s (layer %s -> %s).\n"+
					"Either remove it, or declare it in .arch-baseline with a "+
					"reason and raise the count.", key, fromLayer, toLayer)
			}
		}
	}

	// A baseline entry whose import is gone must not linger as dead debt.
	for key := range baseline {
		if !seen[key] {
			t.Errorf("stale .arch-baseline entry %q — that import no longer "+
				"exists. Remove the line and lower the count.", key)
		}
	}
}

// TestDomainImportsOnlyStdlib closes hole 2.
func TestDomainImportsOnlyStdlib(t *testing.T) {
	for _, p := range loadPackages(t) {
		r := rel(p.ImportPath)
		if r == "" || len(p.GoFiles) == 0 {
			continue
		}
		if l, ok := layerOf(r); !ok || l != "domain" {
			continue
		}
		for _, imp := range p.Imports {
			// Path-boundary aware: a sibling module such as
			// <module>-plugins/foo starts with the module path but is a
			// third-party dependency, and skipping it here would let it into
			// the domain unnoticed.
			if imp == modulePath || strings.HasPrefix(imp, modulePath+"/") {
				continue // internal edges belong to the test above
			}
			// An import path whose first segment contains a dot is a module
			// path; anything else is stdlib.
			first, _, _ := strings.Cut(imp, "/")
			if !strings.Contains(first, ".") {
				continue
			}
			if domainExternalAllowlist[imp] {
				continue
			}
			t.Errorf("domain package %q imports external %q.\n"+
				"The business core stays framework-free — put it behind a port, "+
				"or add it to domainExternalAllowlist with a reason.", r, imp)
		}
	}
}

// loadBaseline reads .arch-baseline and checks its trailing count against the
// number of declared exceptions. The count is the ratchet: raising it is a
// visible, reviewable commit.
func loadBaseline(t *testing.T) map[string]bool {
	t.Helper()
	path := filepath.Join(moduleRoot(t), ".arch-baseline")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening .arch-baseline: %v", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading .arch-baseline: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal(".arch-baseline has no count line")
	}

	want, err := strconv.Atoi(lines[len(lines)-1])
	if err != nil {
		t.Fatalf(".arch-baseline last line must be the count, got %q", lines[len(lines)-1])
	}

	entries := map[string]bool{}
	for _, line := range lines[:len(lines)-1] {
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		from, to, ok := strings.Cut(line, "->")
		if !ok {
			t.Fatalf("malformed .arch-baseline line: %q (want `pkg -> import  # reason`)", line)
		}
		entries[strings.TrimSpace(from)+" -> "+strings.TrimSpace(to)] = true
	}

	if got := len(entries); got != want {
		t.Errorf(".arch-baseline declares %d exceptions but its count says %d — "+
			"the ratchet only works if they agree", got, want)
	}
	return entries
}
