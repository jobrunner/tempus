# Harness-Erzwingung und Architektur-Drift-Schutz — Implementierungsplan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fehlende Harness-Bestandteile und Architektur-Drift werden von Gates erzwungen statt von Aufmerksamkeit — im Skill `new-go-service` für künftige Services, in tempus für den Bestand.

**Architecture:** Drei Erzwingungsebenen. (1) `scripts/harness-check.sh` prüft Existenz **und** Verdrahtung jedes Pflichtartefakts gegen ein Manifest, mit `.harness-waivers` als zählendem Ratchet. (2) `internal/arch/arch_test.go` ist eine deny-by-default-Fitnessfunktion über den Import-Graphen aus `go list -json`, mit `.arch-baseline` als Ratchet für deklarierte Ausnahmen. (3) depguard bleibt schneller Lint-Vorfilter, ergänzt um externe Pakete.

**Tech Stack:** Go 1.26, bash, GitHub Actions, zizmor, Dependabot, gremlins, golangci-lint (depguard), release-please.

**Spec:** `docs/superpowers/specs/2026-09-14-harness-enforcement-design.md`

## Global Constraints

- Modulpfad: `github.com/jobrunner/tempus`. Env-Prefix: `TEMPUS_`.
- Go-Ziel: `go 1.26.0`, `toolchain go1.26.6` — exakt wie ortus/situs/hostus.
- **`unset GOTOOLCHAIN`** vor jedem Go-Aufruf; asdf pinnt sonst 1.24.4.
- `make verify` muss grün sein: fmt-check, vet, lint, test, arch, debt-guard.
- `internal/adapters/http/openapi.yaml` und `api/openapi/openapi.yaml` bleiben **byte-identisch**. Dieser Plan ändert die Spec nicht — falls doch, beide Kopien.
- `.coverage-floors` ist ein Ratchet: pro neuem Paket **mit** Nicht-Test-Code eine Zeile. `internal/arch` enthält nur `_test.go` und bekommt **keine** Zeile (das Gate prüft nur gelistete Pakete).
- `.debt-budget` steht auf `1`. Keine neuen `//nolint`/`#nosec` ohne Gegenrechnung.
- Commits: Conventional Commits, `type-enum` aus `.commitlintrc.yml`, Header ≤120 Zeichen.
- PRs gegen `main`, **Squash-Merge**. `main` ist geschützt (linear history, required checks, Copilot-Review).
- Copilot reviewt jeden PR automatisch. Review abwarten, berechtigte Funde fixen, pushen, iterieren bis sauber. Gelegentlich manuelles Re-Request über `requested_reviewers` nötig.
- Jeder Commit endet mit `Claude-Session: https://claude.ai/code/session_01F3Qu3JdePaALUpcmxXdEgZ`.

## Reihenfolge

Bindend sequenziell. Phase A liefert die Templates, aus denen B–D kopieren.
Phase C **vor** D, damit die Toolchain-Hygiene-Regel die acht hartkodierten
Go-Versionen überhaupt erst als Fehler sichtbar macht.

| Phase | Repo | Ergebnis |
|---|---|---|
| A | `claude-skills` (Submodul) | Skill-PR: Manifest, Templates, Referenzen |
| B | tempus | PR 1 — Supply chain |
| C | tempus | PR 2 — Arch-Gate, Harness, LICENSE, mutation-gate |
| D | tempus | PR 3 — Go 1.26 + `go-version-file` |
| E | tempus | Release |

---

## File Structure

**Phase A — `claude-skills/skills/new-go-service/`**

| Datei | Verantwortung |
|---|---|
| `SKILL.md` (mod) | Manifest-Abschnitt, Red-Flags, Step 9 Pflichtliste, Step 10a |
| `reference/architecture.md` (mod) | Befund „depguard ist eine Denylist", Allowlist/Ratchet |
| `reference/ci-and-release.md` (mod) | Pflicht/Optional-Spalte, Toolchain-Hygiene |
| `reference/ratchets-and-harnesses.md` (mod) | Ratchet 5, Fitnessfunktion Harness-Vollständigkeit |
| `templates/harness-check.sh` (neu) | Das Gate |
| `templates/harness.yml` (neu) | CI-Job dafür |
| `templates/arch_test.go.tmpl` (neu) | Deny-by-default-Fitnessfunktion |
| `templates/arch-baseline` (neu) | Ratchet-Datei, leeres Muster |
| `templates/dependabot.yml` (neu) | 4 Ecosystems |
| `templates/zizmor.yml` (neu) | zizmor-Policy |
| `templates/actions-security.yml` (neu) | Wöchentlicher zizmor-Scan |
| `templates/dependabot-auto-merge.yml` (neu) | Auto-Merge für Non-Major |
| `templates/vuln-scan.yml` (neu) | Wöchentlicher govulncheck |

**Phase B–D — tempus**

| Datei | Verantwortung |
|---|---|
| `.github/dependabot.yml` (neu) | gomod, github-actions, docker, gitsubmodule |
| `.github/zizmor.yml` (neu) | Policy: Tag-Pinning erlaubt |
| `.github/workflows/actions-security.yml` (neu) | zizmor wöchentlich |
| `.github/workflows/dependabot-auto-merge.yml` (neu) | Auto-Merge |
| `internal/arch/arch_test.go` (neu) | Architektur-Fitnessfunktion |
| `.arch-baseline` (neu) | 2 deklarierte Ausnahmen |
| `scripts/harness-check.sh` (neu) | Harness-Gate |
| `.harness-waivers` (neu) | Waiver-Ratchet |
| `scripts/mutation-gate.sh` (neu) | Eine Einsprungstelle für Mutation |
| `.mutation-thresholds` (neu) | Schwellen, aus Makefile+Workflow zusammengeführt |
| `LICENSE` (neu) | MIT, unverändert |
| `.golangci.yml` (mod) | depguard um externe Pakete ergänzt |
| `Makefile` (mod) | `harness`-Target, `arch` erweitert, `mutation` delegiert |
| `.github/workflows/ci.yml` (mod) | Harness-Job; `go-version-file` |
| `.github/workflows/mutation.yml` (mod) | delegiert an Gate; macOS-Kommentar |
| `go.mod`, `Dockerfile`, `README.md` (mod) | Go 1.26, Lizenz-Abschnitt |

---

# Phase A — Skill `new-go-service`

Arbeitsverzeichnis: `.claude/vendor/claude-skills`. Eigenes Repo, eigener
Branch, eigener PR. tempus hält nur den Submodul-Pointer; der wird in Phase C
mitgezogen.

### Task A1: Branch und Templates für Supply chain

**Files:**
- Create: `skills/new-go-service/templates/dependabot.yml`
- Create: `skills/new-go-service/templates/zizmor.yml`
- Create: `skills/new-go-service/templates/actions-security.yml`
- Create: `skills/new-go-service/templates/dependabot-auto-merge.yml`
- Create: `skills/new-go-service/templates/vuln-scan.yml`

**Interfaces:**
- Produces: fünf Template-Dateien, die Phase B nach tempus kopiert. Platzhalter `<svc>`, `<owner>`, `<module>` wie im Skill üblich.

- [ ] **Step 1: Branch anlegen**

```bash
cd .claude/vendor/claude-skills
git checkout main && git pull
git checkout -b feat/harness-manifest-and-arch-gate
```

- [ ] **Step 2: `templates/dependabot.yml` schreiben**

Vorlage ist ortus' Datei (`~/work/projects/ortus/.github/dependabot.yml`),
generalisiert. Pflicht sind vier Ecosystems; `docker` nur wenn ein `Dockerfile`
existiert, `gitsubmodule` nur wenn `.gitmodules` existiert.

```yaml
# Dependabot. Vier Ecosystems; docker/gitsubmodule nur wenn zutreffend.
# Das gomod-Ecosystem bumpt AUCH die `toolchain`-Direktive in go.mod — das ist
# der Mechanismus, der den Service von der nächsten stdlib-CVE fernhält.
# Voraussetzung dafür, dass es auch die CI anhebt: die Workflows lesen ihre
# Go-Version mit `go-version-file: go.mod`, nie hartkodiert.
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule: { interval: weekly, day: monday }
    open-pull-requests-limit: 5
    labels: [dependencies, github-actions]
    groups:
      github-actions:
        patterns: ["*"]

  - package-ecosystem: gomod
    directory: /
    schedule: { interval: weekly, day: monday }
    open-pull-requests-limit: 10
    labels: [dependencies, go]
    groups:
      opentelemetry:
        patterns: ["go.opentelemetry.io/*"]
      go-minor-patch:
        patterns: ["*"]
        update-types: [minor, patch]

  # Nur wenn ein Dockerfile existiert.
  - package-ecosystem: docker
    directory: /
    schedule: { interval: weekly, day: monday }
    open-pull-requests-limit: 3
    labels: [dependencies, docker]

  # Nur wenn .gitmodules existiert.
  - package-ecosystem: gitsubmodule
    directory: /
    schedule: { interval: weekly, day: monday }
    open-pull-requests-limit: 3
    labels: [dependencies, skills]
```

- [ ] **Step 3: `templates/zizmor.yml` schreiben**

```yaml
# zizmor-Policy. Pin-Strategie: Actions per Tag (@v6), Updates über Dependabot.
# SHA-Pinning ist der Goldstandard, deaktiviert aber Dependabots Update-Story
# für Actions. Für ein v0.x-Projekt ist Tag-Pinning + wöchentlicher Bump der
# gewählte Kompromiss; bei verschärftem Bedrohungsmodell neu bewerten.
# https://docs.zizmor.sh/configuration/
rules:
  unpinned-uses:
    config:
      policies:
        "*": ref-pin
  # artipacked meldet jeden checkout ohne persist-credentials:false. Bei v0.x
  # überwiegt das Rauschen; Workflows mit Credentials vergeben explizite
  # permissions und leaken keine Artefakte.
  artipacked:
    disable: true
  dependabot-cooldown:
    disable: true
```

- [ ] **Step 4: `templates/actions-security.yml` schreiben**

Kopie von ortus' `actions-security.yml`, `<svc>`-neutral. Die Kommentare zu den
beiden zizmor-Eigenheiten sind **tragend** und müssen mit:
SARIF-Format exitet immer 0 (deshalb der zweite, autoritative Lauf im
Plain-Format mit Exit 14 als Findings-Signal), und `rc` wird über `env:`
injiziert statt inline expandiert, weil der eigene Audit sonst
Template-Injection meldet.

```bash
cp ~/work/projects/ortus/.github/workflows/actions-security.yml \
   skills/new-go-service/templates/actions-security.yml
```

Danach im Template `ortus` → `<svc>` ersetzen und den Verweis auf `vuln-scan.yml`
als Muster belassen.

- [ ] **Step 5: `templates/dependabot-auto-merge.yml` und `templates/vuln-scan.yml`**

```bash
cp ~/work/projects/ortus/.github/workflows/dependabot-auto-merge.yml \
   skills/new-go-service/templates/dependabot-auto-merge.yml
cp ~/work/projects/ortus/.github/workflows/vuln-scan.yml \
   skills/new-go-service/templates/vuln-scan.yml
```

Im Auto-Merge-Template den Kommentar behalten, warum `pull_request_target`
sicher ist (der Workflow checkt PR-HEAD **nie** aus) und warum
`github.event.pull_request.user.login` statt `github.actor` geprüft wird
(letzteres ist spoofbar).

- [ ] **Step 6: Commit**

```bash
git add skills/new-go-service/templates/
git commit -m "feat(new-go-service): add supply-chain and actions-security templates"
```

### Task A2: Das Harness-Gate

**Files:**
- Create: `skills/new-go-service/templates/harness-check.sh`
- Create: `skills/new-go-service/templates/harness.yml`

**Interfaces:**
- Produces: `scripts/harness-check.sh`, exit 0 grün / 1 bei Lücke / 2 bei Setup-Fehler. Liest `.harness-waivers` (Format: `KEY  # Begründung`, letzte Nicht-Kommentarzeile ist die Zählung).

- [ ] **Step 1: `templates/harness-check.sh` schreiben**

```bash
#!/usr/bin/env bash
# harness-check.sh — Vollständigkeit des Quality-Harness (FITNESSFUNKTION).
#
# Prüft Existenz UND Verdrahtung jedes Pflichtartefakts. Existenz allein genügt
# nicht: eine mutation.yml, die `gremlins unleash ./internal/...` aufruft, ist
# vorhanden und misst nachweislich nichts (die ...-Wildcard wird nicht
# expandiert).
#
# Ausstiegsluke: .harness-waivers, eine Zeile pro bewusst ausgelassenem Punkt
# mit Begründung, plus eine Zählung als letzte Zeile. Die Zählung ist ein
# RATCHET — sie darf nur sinken.
#
# Exit: 0 grün, 1 Lücke, 2 Setup-Fehler.
set -uo pipefail

WAIVERS="${WAIVERS:-.harness-waivers}"
fail=0
declare -a MISSING=()

waived() {
  [ -f "$WAIVERS" ] || return 1
  grep -qE "^[[:space:]]*$1([[:space:]]|#|$)" "$WAIVERS"
}

# check <key> <beschreibung> <test-kommando…>
check() {
  local key="$1" desc="$2"; shift 2
  if "$@" >/dev/null 2>&1; then
    printf '  ok       %-34s %s\n' "$key" "$desc"
  elif waived "$key"; then
    printf '  waived   %-34s %s\n' "$key" "$desc"
  else
    printf '  MISSING  %-34s %s\n' "$key" "$desc"
    MISSING+=("$key"); fail=1
  fi
}

has_file()  { [ -f "$1" ]; }
greps()     { grep -qE "$2" "$1" 2>/dev/null; }
# Kein Workflow darf eine Go-Version hartkodieren.
no_hardcoded_go() {
  ! grep -rlE "^\s*go-version:\s*['\"]?[0-9]" .github/workflows/ 2>/dev/null | grep -q .
}
# mutation.yml darf keine ...-Wildcard an unleash übergeben.
no_wildcard_unleash() {
  ! grep -rE "unleash[^|]*\.\.\." .github/workflows/ Makefile scripts/ 2>/dev/null | grep -q .
}
# Ein `on: release`-Workflow feuert bei release-please nie.
no_on_release() {
  ! grep -rlE "^\s*on:\s*$" -A3 .github/workflows/ 2>/dev/null \
    | xargs -r grep -lE "^\s*release:\s*$" 2>/dev/null | grep -q .
}

echo "harness-check: Pflichtbestandteile"

echo "— Supply chain"
check dependabot          "dependabot.yml vorhanden"        has_file .github/dependabot.yml
check dependabot-gomod    "gomod-Ecosystem"                 greps .github/dependabot.yml 'package-ecosystem:\s*gomod'
check dependabot-actions  "github-actions-Ecosystem"        greps .github/dependabot.yml 'package-ecosystem:\s*github-actions'
[ -f Dockerfile ] && \
check dependabot-docker   "docker-Ecosystem (Dockerfile da)" greps .github/dependabot.yml 'package-ecosystem:\s*docker'
[ -f .gitmodules ] && \
check dependabot-submod   "gitsubmodule-Ecosystem"          greps .github/dependabot.yml 'package-ecosystem:\s*gitsubmodule'
check dependabot-automerge "Auto-Merge-Workflow"            has_file .github/workflows/dependabot-auto-merge.yml

echo "— Actions-Security"
check zizmor-config       "zizmor.yml vorhanden"            has_file .github/zizmor.yml
check zizmor-workflow     "actions-security.yml vorhanden"  has_file .github/workflows/actions-security.yml
check zizmor-wired        "Workflow nutzt die Policy"       greps .github/workflows/actions-security.yml 'config .github/zizmor.yml'

echo "— Ratchets"
check debt-budget         ".debt-budget"                    has_file .debt-budget
check debt-script         "scripts/debt-guard.sh"           has_file scripts/debt-guard.sh
check coverage-floors     ".coverage-floors"                has_file .coverage-floors
check coverage-script     "scripts/coverage-gate.sh"        has_file scripts/coverage-gate.sh
check mutation-thresholds ".mutation-thresholds"            has_file .mutation-thresholds
check mutation-script     "scripts/mutation-gate.sh"        has_file scripts/mutation-gate.sh
check mutation-workflow   "mutation.yml nutzt das Gate"     greps .github/workflows/mutation.yml 'mutation-gate\.sh'
check mutation-vacuity    "kein ...-Wildcard an unleash"    no_wildcard_unleash
check codecharta-baseline ".codecharta-ratchet.json"        has_file .codecharta-ratchet.json
check codecharta-script   "scripts/codecharta-ratchet.py"   has_file scripts/codecharta-ratchet.py
check codecharta-workflow "codecharta.yml"                  has_file .github/workflows/codecharta.yml
check arch-baseline       ".arch-baseline"                  has_file .arch-baseline

echo "— Release und Recht"
check commitlint-config   ".commitlintrc.yml"               has_file .commitlintrc.yml
check commitlint-workflow "commitlint.yml"                  has_file .github/workflows/commitlint.yml
check release-config      "release-please-config.json"      has_file release-please-config.json
check release-initial     "initial-version gesetzt"         greps release-please-config.json 'initial-version'
check release-preminor    "bump-minor-pre-major gesetzt"    greps release-please-config.json 'bump-minor-pre-major'
check goreleaser          ".goreleaser.yml"                 has_file .goreleaser.yml
check license             "LICENSE"                         has_file LICENSE

echo "— Toolchain-Hygiene"
check go-version-file     "keine hartkodierte go-version"   no_hardcoded_go

# Waiver-Ratchet.
if [ -f "$WAIVERS" ]; then
  declared=$(grep -vE '^\s*#|^\s*$' "$WAIVERS" | tail -1 | tr -d ' ')
  actual=$(($(grep -vE '^\s*#|^\s*$' "$WAIVERS" | wc -l) - 1))
  echo "— Waiver: $actual deklariert, Baseline $declared"
  if [ "$actual" -gt "$declared" ]; then
    echo "FAIL: Waiver über Baseline gewachsen ($actual > $declared)"; fail=1
  elif [ "$actual" -lt "$declared" ]; then
    echo "✓ Waiver gesunken — senke die Baseline auf $actual, um es festzuhalten"
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo
  echo "harness-check: FEHLGESCHLAGEN — fehlend: ${MISSING[*]}"
  echo "Entweder das Artefakt anlegen (Templates im Skill new-go-service),"
  echo "oder es mit Begründung in $WAIVERS aufnehmen und die Zählung erhöhen."
  exit 1
fi
echo "harness-check: OK"
```

- [ ] **Step 2: Skript lokal gegen ortus prüfen (soll weitgehend grün sein)**

Run: `cd ~/work/projects/ortus && bash <pfad>/harness-check.sh; echo "rc=$?"`
Expected: Die Supply-chain- und Actions-Security-Zeilen sind `ok`. Abweichungen
notieren — sie zeigen, wo das Manifest zu streng oder zu lax formuliert ist, und
gehören vor dem Commit korrigiert.

- [ ] **Step 3: `templates/harness.yml` schreiben**

```yaml
# Harness-Vollständigkeit. Fällt aus, wenn ein Pflichtbestandteil des
# Quality-Harness fehlt oder nicht verdrahtet ist.
name: Harness
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  harness:
    name: Harness
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Harness completeness
        run: bash scripts/harness-check.sh
```

- [ ] **Step 4: Commit**

```bash
chmod +x skills/new-go-service/templates/harness-check.sh
git add skills/new-go-service/templates/harness-check.sh skills/new-go-service/templates/harness.yml
git commit -m "feat(new-go-service): add harness-completeness gate and its workflow"
```

### Task A3: Die Architektur-Fitnessfunktion

**Files:**
- Create: `skills/new-go-service/templates/arch_test.go.tmpl`
- Create: `skills/new-go-service/templates/arch-baseline`

**Interfaces:**
- Produces: Paket `arch_test` unter `internal/arch/`. Enthält **nur** Testdateien — dadurch listet `go list` es ohne `GoFiles`, und die Vollständigkeitsprüfung überspringt es von selbst, ohne Sonderfall.
- Consumes: `.arch-baseline` im Repo-Root.

- [ ] **Step 1: `templates/arch-baseline` schreiben**

```
# Deklarierte Architektur-Ausnahmen. RATCHET: darf nur schrumpfen.
#
# Jede Zeile: <paket> -> <import>  # begruendung
# Die letzte Nicht-Kommentarzeile ist die Zählung.
#
# Eine Kante, die hier steht, ist eine bewusste, reviewte Abweichung von der
# Schichtregel — kein Freibrief. Verschwindet sie, meldet der Test es und die
# Zählung wird gesenkt.
0
```

- [ ] **Step 2: `templates/arch_test.go.tmpl` schreiben**

```go
// Copy to internal/arch/arch_test.go — this package holds ONLY test files, so
// `go list` reports it without GoFiles and the completeness check below skips
// it without needing a special case.
//
// Architecture fitness function, deny-by-default. depguard (see .golangci.yml)
// is a DENYLIST and therefore has three holes this test closes:
//   1. a new package matching no `files:` rule is unguarded entirely,
//   2. external imports into the domain are not covered at all,
//   3. every new layer must be hand-added to every deny list.
package arch_test

import (
	"encoding/json"
	"os/exec"
	"os"
	"bufio"
	"strings"
	"testing"
)

const modulePath = "<module>"

// layerOf maps a package path to its layer. Order matters: the first matching
// prefix wins, so more specific prefixes come first.
var layerPrefixes = []struct{ prefix, layer string }{
	{"internal/domain", "domain"},
	{"internal/ports", "ports"},
	{"internal/application", "application"},
	{"internal/adapters", "adapters"},
	{"internal/config", "config"},
	{"internal/app", "app"},
	{"cmd/", "cmd"},
}

// allowed[from] = set of layers `from` may import. Empty set = imports nothing
// internal. A layer absent from this map is a bug, not a permission.
var allowed = map[string]map[string]bool{
	"domain":      {},
	"ports":       {"domain": true},
	"config":      {"domain": true},
	"application": {"domain": true, "ports": true},
	"adapters":    {"domain": true, "ports": true},
	"app":         {"domain": true, "ports": true, "application": true, "adapters": true, "config": true},
	"cmd":         {"domain": true, "ports": true, "application": true, "adapters": true, "config": true, "app": true},
}

// domainExternalAllowlist names the non-stdlib imports the domain may use.
// Keep this empty if you can; every entry is a dependency the business core
// carries into every test.
var domainExternalAllowlist = map[string]bool{}

type goPkg struct {
	ImportPath string
	GoFiles    []string
	Imports    []string
}

// loadPackages shells out to `go list`. Deliberately not
// golang.org/x/tools/go/packages: that would be a new go.mod dependency for
// information the toolchain already exposes.
func loadPackages(t *testing.T) []goPkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = "../.."
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
	if len(pkgs) == 0 {
		t.Fatal("go list returned no packages — the gate would be vacuous")
	}
	return pkgs
}

// rel strips the module prefix; returns "" for packages outside the module.
func rel(importPath string) string {
	if importPath == modulePath {
		return ""
	}
	return strings.TrimPrefix(importPath, modulePath+"/")
}

func layerOf(relPath string) (string, bool) {
	for _, lp := range layerPrefixes {
		if relPath == strings.TrimSuffix(lp.prefix, "/") || strings.HasPrefix(relPath, lp.prefix) {
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
			t.Errorf("package %q maps to no layer — add a prefix to layerPrefixes "+
				"(and a matching depguard rule), or move the package", r)
		}
	}
}

// TestImportsRespectLayerBoundaries closes hole 3: the rule is an allowlist, so
// a new layer cannot silently inherit permissions.
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
			continue // reported by TestEveryPackageHasALayer
		}
		for _, imp := range p.Imports {
			to := rel(imp)
			if to == imp || to == "" {
				continue // not an internal import
			}
			toLayer, ok := layerOf(to)
			if !ok {
				continue
			}
			if fromLayer == toLayer {
				continue // intra-layer is fine
			}
			if allowed[fromLayer][toLayer] {
				continue
			}
			key := from + " -> " + to
			seen[key] = true
			if !baseline[key] {
				t.Errorf("forbidden import %s (%s -> %s). Either remove it, or "+
					"declare it in .arch-baseline with a reason and raise the count.",
					key, fromLayer, toLayer)
			}
		}
	}

	for key := range baseline {
		if !seen[key] {
			t.Errorf("stale .arch-baseline entry %q — the import is gone. "+
				"Remove the line and lower the count.", key)
		}
	}
}

// TestDomainImportsOnlyStdlib closes hole 2.
func TestDomainImportsOnlyStdlib(t *testing.T) {
	for _, p := range loadPackages(t) {
		r := rel(p.ImportPath)
		if !strings.HasPrefix(r, "internal/domain") || len(p.GoFiles) == 0 {
			continue
		}
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, modulePath) {
				continue // internal edges are TestImportsRespectLayerBoundaries' job
			}
			// An import path whose first segment carries a dot is a module path;
			// anything else is stdlib.
			first, _, _ := strings.Cut(imp, "/")
			if !strings.Contains(first, ".") {
				continue
			}
			if domainExternalAllowlist[imp] {
				continue
			}
			t.Errorf("domain package %q imports external %q — the business core "+
				"stays framework-free; put it behind a port", r, imp)
		}
	}
}

// loadBaseline reads .arch-baseline and verifies its trailing count matches the
// number of declared exceptions (the ratchet).
func loadBaseline(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open("../../.arch-baseline")
	if err != nil {
		t.Fatalf("opening .arch-baseline: %v", err)
	}
	defer f.Close()

	entries := map[string]bool{}
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

	count := strings.TrimSpace(lines[len(lines)-1])
	for _, line := range lines[:len(lines)-1] {
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		from, to, ok := strings.Cut(line, "->")
		if !ok {
			t.Fatalf("malformed .arch-baseline line: %q", line)
		}
		entries[strings.TrimSpace(from)+" -> "+strings.TrimSpace(to)] = true
	}

	want := strings.TrimSpace(count)
	got := len(entries)
	if want != itoa(got) {
		t.Errorf(".arch-baseline count is %s but %d exceptions are declared — "+
			"the ratchet only works if they agree", want, got)
	}
	return entries
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
```

- [ ] **Step 3: Commit**

```bash
git add skills/new-go-service/templates/arch_test.go.tmpl skills/new-go-service/templates/arch-baseline
git commit -m "feat(new-go-service): add deny-by-default architecture fitness test"
```

### Task A4: SKILL.md und Referenzen

**Files:**
- Modify: `skills/new-go-service/SKILL.md`
- Modify: `skills/new-go-service/reference/architecture.md`
- Modify: `skills/new-go-service/reference/ci-and-release.md`
- Modify: `skills/new-go-service/reference/ratchets-and-harnesses.md`

**Interfaces:**
- Consumes: die Templates aus A1–A3.

- [ ] **Step 1: Manifest-Abschnitt in `SKILL.md` einfügen**

Direkt nach der Tabelle „What you get" einfügen, vor „Procedure — scaffold in
order". Inhalt: die Manifest-Tabelle aus der Spec (Gruppe / Artefakte /
Verdrahtungsprüfung), eingeleitet mit:

> ## Das Manifest — nicht verhandelbar
>
> Die folgenden Artefakte sind **Pflicht**, unabhängig von Projektgröße, Reifegrad
> oder Zeitdruck. Sie werden von `scripts/harness-check.sh` erzwungen, das in
> `make verify` und als eigener CI-Job läuft. Ein Service, bei dem dieses Gate
> nicht grün ist, ist nicht fertig.
>
> Die Ausstiegsluke ist `.harness-waivers`: eine Zeile pro bewusst ausgelassenem
> Punkt, mit Begründung, plus eine Zählung, die nur sinken darf. Auslassen ist
> erlaubt — stillschweigend auslassen nicht.

- [ ] **Step 2: Red-Flags-Tabelle anhängen**

```markdown
### Red Flags beim Weglassen

| Gedanke | Wirklichkeit |
|---|---|
| „Nur ein kleiner interner Service" | Das Manifest skaliert nicht mit der Projektgröße. Der Aufwand ist einmalig, die Lücke dauerhaft. |
| „Dependabot machen wir später" | Ohne Dependabot bewegt sich `go.mod` nur, wenn ein Feature es anfasst. Gemessen: tempus blieb so drei Releases lang auf einer alten Go-Minor, während vier Schwesterservices anhoben. |
| „zizmor ist doch nur für öffentliche Repos" | Script-Injection über `github.event`-Felder und pwn-request-Muster sind unabhängig von der Sichtbarkeit. |
| „Mutation-Testing ist zu langsam" | Dann schwellenweise pro Paket — nicht gar nicht. Ein fehlendes Gate misst nichts, ein enges Gate misst etwas. |
| „depguard prüft die Architektur doch schon" | depguard ist eine Denylist. Ein Paket, das keine Regel matcht, ist ungeschützt. Siehe `reference/architecture.md`. |
| „Die Workflows haben die Go-Version doch drin" | Genau das ist der Fehler. Hartkodiert heißt: ein `go.mod`-Bump zieht die CI nicht mit. |
```

- [ ] **Step 3: Step 9 in `SKILL.md` auf die Pflichtliste umstellen**

Die Aufzählung „(ci, mutation, commitlint, release-please, security)" wird zur
vollständigen Pflichtliste: `ci.yml`, `mutation.yml`, `commitlint.yml`,
`release-please.yml`, `codecharta.yml`, `harness.yml`, `actions-security.yml`,
`vuln-scan.yml`, `dependabot-auto-merge.yml`, plus `.github/dependabot.yml` und
`.github/zizmor.yml`. Mit dem Satz: „Diese Liste ist das Minimum, kein Menü."

Zusätzlich der Toolchain-Absatz:

> **Die Go-Version gehört in genau eine Datei.** Jeder Workflow liest sie mit
> `go-version-file: go.mod`; ein hartkodiertes `go-version: '1.x'` ist ein
> Fehler, den `harness-check.sh` meldet. Der Grund ist nicht Ästhetik: Dependabots
> gomod-Ecosystem bumpt die `toolchain`-Direktive in `go.mod`, und nur wenn die
> Workflows von dort lesen, zieht die CI beim Bump mit. Gemessen: ein Service
> dieser Familie trug die Version achtmal hartkodiert und wäre auch nach einem
> go.mod-Bump auf der alten Toolchain gebaut worden.

- [ ] **Step 4: Step 10a ergänzen**

> **Step 10a — Harness abnehmen.** `make harness` muss grün sein. Dieses Gate ist
> die Abnahme: solange es rot ist, fehlt dem Service ein Teil seines
> Immunsystems, egal wie grün die Tests sind.

- [ ] **Step 5: `reference/architecture.md` erweitern**

Neuer Abschnitt nach „Dependency direction (the boundary rule)", der die drei
Löcher aus der Spec (Befund 1) benennt und die Allowlist/Ratchet-Konstruktion
beschreibt. Kernsatz:

> depguard ist ein **Vorfilter**, keine Absicherung. Es prüft, was es kennt.
> Die Absicherung ist `internal/arch/arch_test.go`: es klassifiziert jedes Paket,
> behandelt ein unklassifiziertes Paket als Fehler und erlaubt nur benannte
> Kanten. Deklarierte Ausnahmen stehen mit Begründung in `.arch-baseline` und
> unterliegen einem Ratchet.

- [ ] **Step 6: `reference/ci-and-release.md` — Pflicht/Optional-Spalte**

Die Workflow-Tabelle bekommt eine dritte Spalte. Pflicht: `ci.yml`,
`mutation.yml`, `codecharta.yml`, `commitlint.yml`, `release-please.yml`,
`harness.yml`, `actions-security.yml`, `vuln-scan.yml`,
`dependabot-auto-merge.yml`. Optional: `openapi-diff.yml` (nur mit OpenAPI-Spec),
`fuzz.yml`. Die Einleitung „A representative set" wird ersetzt durch:

> Die folgende Tabelle ist **kein Menü**. Die als Pflicht markierten Workflows
> gehören zum Manifest und werden von `harness-check.sh` erzwungen.

- [ ] **Step 7: `reference/ratchets-and-harnesses.md` — Ratchet 5 + Fitnessfunktion**

Ratchet 5 (Architektur-Baseline) analog zu Ratchet 1 dokumentieren: Dateiformat,
Gate-Logik, und der Hinweis, dass eine *verschwundene* Ausnahme ebenfalls
gemeldet wird, damit die Baseline nicht als Altlast weiterlebt. Dazu die
Fitnessfunktion „Harness-Vollständigkeit" mit dem Verdrahtungs-Argument
(Existenz ≠ Wirksamkeit, Beispiel `unleash ./...`).

- [ ] **Step 8: Commit**

```bash
git add skills/new-go-service/
git commit -m "docs(new-go-service): make the harness manifest mandatory and enforced"
```

### Task A5: Skill-PR

- [ ] **Step 1: Push und PR**

```bash
cd .claude/vendor/claude-skills
git push -u origin feat/harness-manifest-and-arch-gate
gh pr create --title "feat(new-go-service): enforce the harness manifest and architecture boundaries" \
  --body "$(cat <<'EOF'
## Warum

tempus blieb als einziger Service der Familie auf Go 1.25, weil ihm Dependabot,
zizmor und eine nicht hartkodierte Go-Version fehlten — alles Dinge, die dieser
Skill zwar beschreibt, aber als "representative set" anbietet statt fordert.

## Was

- **Manifest** in SKILL.md: Pflichtartefakte, Red-Flags, Step 10a als Abnahme.
- **harness-check.sh**: prüft Existenz UND Verdrahtung, `.harness-waivers` als
  zählender Ratchet.
- **arch_test.go**: deny-by-default-Fitnessfunktion. Schließt die drei Löcher
  von depguard als Denylist (unklassifizierte Pakete, externe Domain-Imports,
  handgepflegte Deny-Listen), plus `.arch-baseline` als Ratchet.
- **Supply-chain-Templates**: dependabot, zizmor, actions-security,
  auto-merge, vuln-scan.
- **Toolchain-Hygiene**: `go-version-file: go.mod` ist Pflicht, hartkodierte
  Versionen sind ein Gate-Fehler.

https://claude.ai/code/session_01F3Qu3JdePaALUpcmxXdEgZ
EOF
)"
```

- [ ] **Step 2: Review-Loop und Merge**

Auf Review warten, berechtigte Funde fixen, pushen, iterieren. Dann mergen.

---

# Phase B — tempus PR 1: Supply chain

Branch `feat/harness-supply-chain` existiert bereits mit den zwei Spec-Commits.

### Task B1: Dependabot

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Produces: vier Ecosystems. tempus hat `Dockerfile` **und** `.gitmodules`, also sind alle vier zutreffend.

- [ ] **Step 1: Datei aus dem Skill-Template anlegen**

Template aus Task A1 Step 2 kopieren, dann tempus-spezifisch anpassen: die
`gomod`-Gruppen sind `opentelemetry` (tempus nutzt OTel) und der
`go-minor-patch`-Catch-all. Kein AWS/Azure — tempus hat keine Cloud-SDKs.
Das `gitsubmodule`-Ecosystem zielt auf `.claude/vendor/claude-skills`.

- [ ] **Step 2: YAML validieren**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/dependabot.yml')); print('ok')"`
Expected: `ok`

- [ ] **Step 3: Commit**

```bash
git add .github/dependabot.yml
git commit -m "ci: add dependabot for gomod, actions, docker and the skills submodule"
```

### Task B2: zizmor und Actions-Security

**Files:**
- Create: `.github/zizmor.yml`
- Create: `.github/workflows/actions-security.yml`
- Create: `.github/workflows/dependabot-auto-merge.yml`

- [ ] **Step 1: Die drei Dateien aus den Templates anlegen**

`ortus` → `tempus` ersetzen. Die tragenden Kommentare (SARIF exitet immer 0;
`rc` per `env:` statt inline; `pull_request_target` ist hier sicher, weil
PR-HEAD nie ausgecheckt wird) bleiben erhalten.

- [ ] **Step 2: zizmor lokal gegen tempus laufen lassen**

Run: `uvx zizmor==1.25.2 --config .github/zizmor.yml --min-severity=high . ; echo "rc=$?"`
Expected: `rc=0`. Bei `rc=14` sind echte Funde da — die gehören **in diesem PR**
gefixt, sonst ist der wöchentliche Job ab Tag 1 rot. Typische Funde in
Bestandsworkflows: fehlende `permissions:`-Blöcke, inline expandierte
`${{ }}`-Ausdrücke in `run:`.

- [ ] **Step 3: Commit**

```bash
git add .github/zizmor.yml .github/workflows/actions-security.yml .github/workflows/dependabot-auto-merge.yml
git commit -m "ci: add zizmor actions-security scan and dependabot auto-merge"
```

### Task B3: PR 1 eröffnen, reviewen, mergen

- [ ] **Step 1: `make verify` grün**

Run: `unset GOTOOLCHAIN && make verify`
Expected: alle Stufen grün. Dieser PR fasst keinen Go-Code an; die Prüfung
schützt vor unbeabsichtigten Nebenwirkungen.

- [ ] **Step 2: Push und PR**

```bash
git push -u origin feat/harness-supply-chain
gh pr create --title "ci: add dependabot, zizmor actions-security and auto-merge" --body "..."
```

Der Body nennt den Befund (tempus als einziger Service ohne Dependabot, daher
auf Go 1.25 zurückgeblieben) und verweist auf die Spec im selben PR.

- [ ] **Step 3: Copilot-Review abwarten, fixen, iterieren**

- [ ] **Step 4: Squash-Merge**

```bash
gh pr merge --squash
```

- [ ] **Step 5: Prüfen, dass Dependabot greift**

Run: `gh api repos/jobrunner/tempus/dependabot/alerts --jq 'length'` sowie ein
Blick auf Insights → Dependency graph → Dependabot. Erwartung: die vier
Ecosystems werden gelistet. Dependabot braucht bis zu 24 h für den ersten Lauf;
ein manueller „Check for updates" beschleunigt es.

---

# Phase C — tempus PR 2: Arch-Gate, Harness, LICENSE, mutation-gate

Branch: `feat/arch-gate-and-harness`, von aktualisiertem `main`.

### Task C1: LICENSE und README-Lizenzabschnitt

**Files:**
- Create: `LICENSE`
- Modify: `README.md`

- [ ] **Step 1: MIT-LICENSE anlegen**

Unveränderter MIT-Text, Jahr `2026`, Inhaber `Jo Brunner`. **Keine
Zusatzklauseln** — die Begründung steht in der Spec unter „Entscheidungen".

- [ ] **Step 2: README-Abschnitt „Lizenz" anhängen**

```markdown
## Lizenz

Drei Ebenen, die nicht vermischt werden dürfen:

| Ebene | Regelung |
|---|---|
| Der Dienst (dieser Go-Code) | [MIT](LICENSE) |
| Die Algorithmen | Urheber-Attribution, keine Lizenz |
| Die Daten | Lizenz der jeweiligen Quelle |

MIT deckt **ausschließlich** den Code dieses Repositorys. Die verwendeten
Verfahren und Daten gehören ihren Urhebern und Anbietern: Magnus-Tetens mit
Sonntag-1990-Koeffizienten für den Taupunkt, die WorldClim-Definitionen und
Köppen-Geiger für die BIO-Variablen, ERA5 über Copernicus/ECMWF und Open-Meteo
für die Wetterdaten.

Diese Zuschreibung ist nicht nur dokumentiert, sondern erzwungen: jedes Feature
trägt einen `license`-Block (`name`, `url`, `attribution`), und ein Feature ohne
vollständige Lizenz wird an der Port-Grenze abgewiesen. Wer tempus-Antworten
weiterverwendet, übernimmt diese Angaben mit.
```

- [ ] **Step 3: Commit**

```bash
git add LICENSE README.md
git commit -m "docs: add MIT licence for the code and separate the three licence layers"
```

### Task C2: Mutation-Gate zusammenführen

**Files:**
- Create: `scripts/mutation-gate.sh`
- Create: `.mutation-thresholds`
- Modify: `Makefile:mutation`
- Modify: `.github/workflows/mutation.yml`

**Interfaces:**
- Produces: `scripts/mutation-gate.sh` als **einzige** Einsprungstelle. `make mutation` und der Workflow rufen beide nur noch dieses Skript, damit lokal und CI nicht divergieren können.

- [ ] **Step 1: `.mutation-thresholds` aus den zwei bestehenden Kopien anlegen**

Die Werte sind heute in Makefile **und** Workflow identisch hinterlegt; sie
werden hierher zusammengeführt.

```
# Per-Paket-Schwellen für gremlins. RATCHET: nur anheben.
# paket                     efficacy  mcover
internal/domain             90        95
internal/application        77        94
```

- [ ] **Step 2: `scripts/mutation-gate.sh` schreiben**

Pflicht: Pakete **explizit** aufzählen (nie `...`), und bei einem Paket ohne
Mutanten **fehlschlagen**, sofern nicht `NONE` deklariert ist — ohne diese
Vakuitätsprüfung ist ein bestandenes Gate von einem nicht messenden nicht zu
unterscheiden.

```bash
#!/usr/bin/env bash
# mutation-gate.sh — gremlins pro Paket gegen .mutation-thresholds.
# EINZIGE Einsprungstelle: `make mutation` und die CI rufen nur dieses Skript.
#
# Zwei Fallen, gegen die hier ausdrücklich konstruiert wird:
#  1. `gremlins unleash ./internal/...` expandiert die Wildcard NICHT, erzeugt
#     null Mutanten und exitet 0 — ein Gate, das nichts misst und grün meldet.
#     Deshalb: Pakete explizit, eines pro Aufruf, plus Vakuitätsprüfung.
#  2. TIMED OUT zählt als KILLED und schönt die Efficacy. Bei Timeouts den
#     timeout-coefficient anheben, DANN die Zahl lesen.
set -uo pipefail
THRESHOLDS="${THRESHOLDS:-.mutation-thresholds}"
[ -f "$THRESHOLDS" ] || { echo "mutation-gate: $THRESHOLDS fehlt" >&2; exit 2; }

fail=0
while read -r pkg eff mcov; do
  [ -z "${pkg:-}" ] && continue
  case "$pkg" in \#*) continue ;; esac
  echo "===== gremlins: $pkg (min efficacy ${eff}%, min mcover ${mcov}%) ====="
  log=$(mktemp)
  if ! gremlins unleash --threshold-efficacy "$eff" --threshold-mcover "$mcov" "./$pkg" 2>&1 | tee "$log"; then
    fail=1
  fi
  if [ "$eff" != "NONE" ] && grep -q "No results to report" "$log"; then
    echo "FAIL: $pkg erzeugte keine Mutanten — das Gate misst hier nichts."
    fail=1
  fi
  rm -f "$log"
done < <(grep -vE '^\s*#|^\s*$' "$THRESHOLDS")

exit $fail
```

- [ ] **Step 3: Makefile und Workflow auf das Skript umstellen**

`Makefile`: das `mutation`-Target ruft nur noch
`gremlins`-Install plus `bash scripts/mutation-gate.sh`. Der Kommentar
„ubuntu only — gremlins panics on macOS" wird ersetzt durch: „läuft auch auf
macOS (gemessen, darwin/arm64) — lokal fahren, nicht nur in CI".

`.github/workflows/mutation.yml`: der `run:`-Block wird zu
`bash scripts/mutation-gate.sh`. Derselbe Kommentar-Fix.

- [ ] **Step 4: Gate lokal fahren**

Run: `unset GOTOOLCHAIN && chmod +x scripts/mutation-gate.sh && make mutation`
Expected: beide Pakete melden echte Ergebnisse, kein „No results to report",
Schwellen gehalten. Bei TIMED-OUT-Zeilen `timeout-coefficient` in
`.gremlins.yaml` anheben und erneut messen.

- [ ] **Step 5: Commit**

```bash
git add scripts/mutation-gate.sh .mutation-thresholds Makefile .github/workflows/mutation.yml
git commit -m "ci: give mutation testing one entrypoint and a vacuity guard"
```

### Task C3: Architektur-Fitnessfunktion

**Files:**
- Create: `internal/arch/arch_test.go`
- Create: `.arch-baseline`
- Modify: `Makefile:arch`

**Interfaces:**
- Consumes: Template aus Task A3.
- Produces: `make arch` schlägt bei verbotener Kante fehl.

- [ ] **Step 1: `.arch-baseline` mit den zwei bekannten Ausnahmen anlegen**

```
# Deklarierte Architektur-Ausnahmen. RATCHET: darf nur schrumpfen.
# Jede Zeile: <paket> -> <import>  # begruendung
# Die letzte Nicht-Kommentarzeile ist die Zählung.
internal/adapters/metrics -> internal/config    # OTel-Meter-Konfig; gehoert hinter einen Port
internal/adapters/telemetry -> internal/config  # dito, Tracer-Setup
2
```

- [ ] **Step 2: Test aus dem Template anlegen, `<module>` ersetzen**

```bash
mkdir -p internal/arch
cp .claude/vendor/claude-skills/skills/new-go-service/templates/arch_test.go.tmpl internal/arch/arch_test.go
sed -i '' 's|<module>|github.com/jobrunner/tempus|' internal/arch/arch_test.go
```

Den `// Copy to …`-Kopfkommentar entfernen.

- [ ] **Step 3: Test laufen lassen — muss GRÜN sein**

Run: `unset GOTOOLCHAIN && go test ./internal/arch/ -v`
Expected: alle vier Tests PASS. Der Graph ist sauber; genau die zwei
Baseline-Kanten existieren.

- [ ] **Step 4: Den Test gegen eine echte Verletzung prüfen**

Sonst ist unbewiesen, dass er etwas fängt. Temporär in
`internal/domain/feature.go` ein `import "github.com/spf13/viper"` einfügen
(plus eine Nutzung, damit es kompiliert), Test laufen lassen.

Run: `unset GOTOOLCHAIN && go test ./internal/arch/ -run TestDomainImportsOnlyStdlib`
Expected: FAIL mit „domain package … imports external …". Danach die Änderung
**vollständig zurücknehmen** (`git checkout -- internal/domain/feature.go`).

- [ ] **Step 5: `make arch` erweitern**

Das Target führt zusätzlich `go test ./internal/arch/` aus, nach depguard und
`go mod tidy -diff`.

- [ ] **Step 6: Commit**

```bash
git add internal/arch/arch_test.go .arch-baseline Makefile
git commit -m "test(arch): add deny-by-default hexagonal fitness function"
```

### Task C4: depguard verschärfen

**Files:**
- Modify: `.golangci.yml`

- [ ] **Step 1: Externe Pakete in der `domain`-Regel verbieten**

Der `domain`-Regel werden Deny-Einträge für die Frameworks hinzugefügt, die
tempus einsetzt und die im Kern nichts zu suchen haben:

```yaml
            - pkg: net/http
              desc: domain stays transport-free; HTTP belongs in the adapter
            - pkg: github.com/gorilla/mux
              desc: domain stays framework-free
            - pkg: github.com/spf13/viper
              desc: domain stays config-free
            - pkg: go.opentelemetry.io
              desc: domain stays telemetry-free; use the Tracer port
```

- [ ] **Step 2: Lint laufen lassen**

Run: `unset GOTOOLCHAIN && make lint`
Expected: grün. Bei einem Treffer ist eine echte Verletzung gefunden — dann
`internal/arch` hat sie übersehen (Allowlist prüfen) oder es ist eine legitime
Ausnahme (Baseline).

- [ ] **Step 3: Commit**

```bash
git add .golangci.yml
git commit -m "ci: deny transport, config and telemetry imports in the domain"
```

### Task C5: Harness-Gate in tempus

**Files:**
- Create: `scripts/harness-check.sh`
- Create: `.harness-waivers`
- Create: `.github/workflows/harness.yml`
- Modify: `Makefile`

- [ ] **Step 1: Skript und Workflow aus den Templates kopieren**

- [ ] **Step 2: Gate laufen lassen und die Lücken sehen**

Run: `bash scripts/harness-check.sh; echo "rc=$?"`
Expected: `rc=1`. Offen sein sollten nur noch: `go-version-file` (acht
hartkodierte Versionen — das ist **PR 3**) und ggf. `vuln-scan`, weil tempus
govulncheck in `security.yml` statt in `vuln-scan.yml` fährt.

- [ ] **Step 3: `.harness-waivers` anlegen**

Nur für das, was bewusst anders gelöst ist. Die Go-Version wird **nicht**
gewaivert — sie wird in PR 3 gefixt; bis dahin ist der Job rot, was korrekt ist
und in der PR-Beschreibung angekündigt wird.

```
# Bewusst ausgelassene Manifest-Punkte, mit Begründung.
# Die letzte Nicht-Kommentarzeile ist die Zählung. RATCHET: nur senken.
go-version-file  # wird in PR 3 behoben (Go 1.26 + go-version-file); Waiver dort entfernen
1
```

- [ ] **Step 4: Gate erneut laufen lassen**

Run: `bash scripts/harness-check.sh; echo "rc=$?"`
Expected: `rc=0`, die `go-version-file`-Zeile als `waived`.

- [ ] **Step 5: `make harness` anlegen und in `verify` aufnehmen**

```makefile
harness: ## Harness completeness (FITNESSFUNKTION)
	@bash scripts/harness-check.sh
```

`verify` wird zu: `fmt-check vet lint test arch debt-guard harness`.

- [ ] **Step 6: Commit**

```bash
chmod +x scripts/harness-check.sh
git add scripts/harness-check.sh .harness-waivers .github/workflows/harness.yml Makefile
git commit -m "ci: enforce harness completeness with a waiver ratchet"
```

### Task C6: Submodul-Pointer und PR 2

- [ ] **Step 1: Submodul auf den gemergten Skill-Stand ziehen**

```bash
cd .claude/vendor/claude-skills && git checkout main && git pull && cd -
git add .claude/vendor/claude-skills
git commit -m "chore(deps): bump claude-skills to the harness-manifest version"
```

- [ ] **Step 2: `make verify` grün**

Run: `unset GOTOOLCHAIN && make verify`
Expected: grün inklusive `harness`.

- [ ] **Step 3: PR eröffnen, reviewen, squash-mergen**

Der PR-Body kündigt ausdrücklich an, dass die Toolchain-Hygiene-Regel bis PR 3
gewaivert ist.

---

# Phase D — tempus PR 3: Go 1.26

Branch: `feat/go-1-26`, von aktualisiertem `main`.

### Task D1: go.mod, Dockerfile, README

**Files:**
- Modify: `go.mod:3,5`
- Modify: `Dockerfile:10`
- Modify: `README.md:96`

- [ ] **Step 1: go.mod anheben**

```bash
unset GOTOOLCHAIN
go mod edit -go=1.26.0 -toolchain=go1.26.6
go mod tidy
```

- [ ] **Step 2: Tests und Build**

Run: `unset GOTOOLCHAIN && go build ./... && go test ./...`
Expected: grün.

- [ ] **Step 3: Dockerfile-Basisimage anheben**

Aktuelles Digest für `golang:1.26.6-alpine` ermitteln und **mit SHA pinnen**
(die bestehende Zeile pinnt bereits, das Muster bleibt):

```bash
docker pull golang:1.26.6-alpine
docker inspect --format='{{index .RepoDigests 0}}' golang:1.26.6-alpine
```

- [ ] **Step 4: README Zeile 96**

„Gebaut mit Go 1.25" → „Gebaut mit Go 1.26".

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum Dockerfile README.md
git commit -m "build: raise Go to 1.26 to match the sibling services"
```

### Task D2: Hartkodierte Go-Versionen entfernen

**Files:**
- Modify: `.github/workflows/ci.yml` (7 Stellen)
- Modify: `.github/workflows/security.yml:21`
- Modify: `.github/workflows/openapi.yml:32`
- Modify: `.github/workflows/release-please.yml:49`
- Modify: `.github/workflows/codecharta.yml` (`GO_VERSION`-Env)

- [ ] **Step 1: Alle `go-version: '1.25'` durch `go-version-file: go.mod` ersetzen**

```bash
grep -rlE "go-version: '1\.25'" .github/workflows/ \
  | xargs sed -i '' "s|go-version: '1\.25'|go-version-file: go.mod|"
```

`codecharta.yml` nutzt `${{ env.GO_VERSION }}` — dort die Env-Variable entfernen
und ebenfalls auf `go-version-file: go.mod` umstellen.

- [ ] **Step 2: Prüfen, dass keine hartkodierte Version übrig ist**

Run: `grep -rnE "go-version:\s*['\"]?[0-9]" .github/workflows/ ; echo "rc=$?"`
Expected: keine Treffer (`rc=1`).

- [ ] **Step 3: Waiver entfernen**

`.harness-waivers` wird auf die leere Form reduziert:

```
# Bewusst ausgelassene Manifest-Punkte, mit Begründung.
# Die letzte Nicht-Kommentarzeile ist die Zählung. RATCHET: nur senken.
0
```

- [ ] **Step 4: Harness-Gate muss jetzt ohne Waiver grün sein**

Run: `bash scripts/harness-check.sh; echo "rc=$?"`
Expected: `rc=0`, `go-version-file` als `ok`, Waiver-Zählung `0`.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ .harness-waivers
git commit -m "ci: read the Go version from go.mod instead of hardcoding it"
```

### Task D3: PR 3

- [ ] **Step 1: `make verify` grün**

Run: `unset GOTOOLCHAIN && make verify`

- [ ] **Step 2: PR eröffnen**

Body nennt: Gleichstand mit ortus/situs/hostus, und dass die acht hartkodierten
Versionen die eigentliche Ursache waren — ohne sie hätte ein Dependabot-Bump die
CI nicht mitgezogen.

- [ ] **Step 3: CI genau prüfen**

Erwartung: alle Jobs laufen auf 1.26. Im Log eines Jobs die Go-Version
verifizieren, nicht nur den grünen Haken lesen.

- [ ] **Step 4: Copilot-Review, fixen, squash-mergen**

---

# Phase E — Release

### Task E1: Release schneiden und verifizieren

- [ ] **Step 1: Auf den release-please-PR warten**

Nach dem Merge von PR 3 öffnet release-please einen `chore(main): release x.y.z`
PR. Die Commits dieser drei PRs sind `ci:`, `build:`, `docs:`, `test:` — **keine
`feat:`**. Nach Conventional Commits ergibt das einen **Patch-Bump**
(0.24.0 → 0.24.1), keinen Minor.

- [ ] **Step 2: Falls kein Release-PR entsteht**

Die Squash-Body-Falle prüfen: Repo-Setting muss `PR_TITLE`+`BLANK` sein
(`gh api repos/jobrunner/tempus --jq '.squash_merge_commit_message'`). Leere
Commits funktionieren **nicht** als Trigger.

- [ ] **Step 3: Release-PR-Baum lokal verifizieren**

Der Release-PR bekommt **keine CI-Checks** (release-please nutzt `GITHUB_TOKEN`).
Deshalb lokal prüfen:

```bash
git fetch origin release-please--branches--main
git checkout release-please--branches--main
unset GOTOOLCHAIN && make verify
bash scripts/openapi-mirror-check.sh
```

Expected: grün, und die beiden OpenAPI-Kopien byte-identisch — release-please
schreibt das `version:`-Feld in beiden um.

- [ ] **Step 4: Release-PR mergen und das Image verifizieren**

```bash
docker manifest inspect ghcr.io/jobrunner/tempus:<version>
docker run --rm -p 8080:8080 ghcr.io/jobrunner/tempus:<version>
curl -sS localhost:8080/health
```

Expected: beide Plattformen im Manifest, Health antwortet. Ein nie gezogenes
Image ist ein unverifiziertes Ergebnis.

---

## Self-Review

**Spec-Abdeckung.** Teil 1 (Manifest+Gate) → A2, C5. Teil 2 (Arch-Gate, drei
Schichten) → A3, C3, C4. Teil 3 (Skill) → A1–A5. Teil 4 (drei PRs) → B, C, D.
Befund 3 (LICENSE, mutation-gate, macOS-Kommentar) → C1, C2. Lizenzentscheidung
inkl. README-Abgrenzung → C1. Release → E1. Keine Lücke.

**Offene Risiken, bewusst in Kauf genommen:**

1. **zizmor kann in B2 rot sein.** Bestandsworkflows haben oft keine
   `permissions:`-Blöcke. Step 2 in B2 fängt das vor dem Merge ab; die Fixes
   gehören in denselben PR.
2. **`make arch` wird langsamer**, weil `go test ./internal/arch/` einen
   `go list ./...`-Lauf kostet. Messbar unter einer Sekunde, akzeptabel.
3. **`no_on_release` in `harness-check.sh` ist heuristisch** (YAML-Struktur per
   grep). Es kann einen `on: release`-Workflow übersehen. Es ist ein Zusatznetz,
   nicht die Absicherung — die steht in der Skill-Referenz.
4. **Der Release ist ein Patch-Bump**, kein Minor. Wenn ein Minor gewünscht ist,
   braucht es einen `feat:`-Commit; keiner dieser drei PRs rechtfertigt einen.
