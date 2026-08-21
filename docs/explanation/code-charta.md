# CodeCharta map and complexity ratchet

`make codecharta` builds a [CodeCharta](https://maibornwolff.github.io/codecharta/)
map of the Go code — a "code city" in which every file is a building whose
footprint, height and colour carry structure, complexity, test coverage and git
history. Load the resulting `tempus.cc.json.gz` in the
[visualization](https://maibornwolff.github.io/codecharta/visualization/); the
CodeCharta CI job attaches the same file to every run as an artifact.

Three sources are merged into one map:

| Source | Contributes |
|---|---|
| `ccsh unifiedparser` | files, LOC, per-file and per-function complexity |
| `go test -coverprofile` → lcov → `ccsh coverageimport` | line coverage per file |
| `ccsh gitlogparser` | churn, number of authors, file age |

Vendored skill templates under `.claude/` are excluded — they are other
projects' code and would otherwise distort both the map and the gates.

Running it locally needs Node and a JRE, because `ccsh` is a JVM tool. The
Makefile uses an installed `ccsh` if there is one and otherwise falls back to
`npx --yes codecharta-analysis@$(CCSH_VERSION)`, so nothing is installed
globally and no admin rights are needed.

## The map is also a gate

CodeCharta only visualises; it has no notion of "fail when worse". That is what
`scripts/codecharta-ratchet.py` adds, checking the merged map against
`.codecharta-ratchet.json`. It covers what the other gates do not: per-package
coverage has its own floors in `.coverage-floors`, and mutation testing is the
separate gremlins gate.

**1. Per-file complexity.** The sum of function complexity per file. Files in
the baseline are frozen at their recorded value, so they cannot grow; every
other file must stay at or below `default_cap` (25). This stops a file becoming
a monolith — but it can be satisfied by splitting a file, since the sum moves
with the code.

**2. Per-function complexity.** The most complex single function in a file
(McCabe), `default_cap` 10. This is the ceiling that cannot be gamed by moving
code around: a function keeps its complexity wherever it lives, so the only way
to pass is to genuinely simplify it or extract cohesive sub-functions.

**3. Hotspots.** A file that is both complex (≥ 30) and under-tested (< 75 %
line coverage) fails the build unless it is grandfathered in `hotspot.allow`.
This blocks *new* complex-and-untested files.

The gate refuses to pass vacuously. If a cap metric is missing from every file,
or if no file carries coverage at all — the coverage import is best-effort, so a
broken test suite could leave the hotspot gate with nothing to judge — it fails
with an explanation instead of reporting a green check that verified nothing.

## Today's baselines

Every number in `.codecharta-ratchet.json` was measured on this repository, not
carried over from elsewhere. The 42-odd non-test Go files have a median
complexity of around 10; these are the outliers still carrying a baseline:

| File | File complexity | Worst function | Line coverage |
|---|---|---|---|
| `internal/adapters/aggregate/aggregate.go` | 51 | 7 | 87.4 % |
| `internal/adapters/bioclim/bioclim.go` | 48 | 9 | 85.5 % |
| `internal/domain/koppen.go` | 45 | 10 | 91.1 % |
| `internal/domain/lunar.go` | 39 | 8 | 96.4 % |
| `internal/domain/bioclim.go` | 35 | 6 | 100 % |
| `internal/adapters/http/server.go` | 33 | 5 | **65.3 %** |
| `internal/adapters/openmeteo/feature.go` | 29 | 7 | 90.9 % |

Coverage here is per file as the map records it, which is not the same as the
per-package figures the coverage floors use.

`http/server.go` is the one remaining grandfathered hotspot: complex and
under-tested. It is recorded as visible debt, not as an exemption.

**The per-function baseline is empty.** No function in the repository exceeds the
cap of 10 any more, so adding an entry there means a function was let past the
cap and needs a reason in review.

### What the first round of ratcheting changed

The gate's first two targets were paid off rather than baselined:

| | Before | After |
|---|---|---|
| `openmeteo.go` file / worst function | 47 / **17** | 23 / 8 |
| `app.go` file / worst function | 30 / **15** | 18 / 7 |

`toFeature` became `hourIndex` + `hourlyValues` + `enrichWeatherCode` +
`enrichBeaufort` in a new `feature.go`; `app.New`'s provider wiring and
observability setup moved to `providers.go` and `observability.go`. Both files
fell below the per-file cap, so their baselines are gone entirely.

Two honest caveats. Splitting a function costs a little total file complexity
(each new function brings its own base), which is why `feature.go` is baselined
at 29 — the win is the per-function maximum, which cannot be moved around. And
`app.go` left the hotspot list because its complexity fell below the threshold,
not because it got tested; its coverage is in fact lower now that the
well-covered wiring moved out. That debt is real, and paying it is the next step
rather than something the gate resolved.

## Ratcheting

The ratchet only turns one way. When a file is simplified, the gate prints a
notice with the new number — move the baseline **down** to lock the improvement
in. Raising a baseline entry is possible but needs a justification in review,
the same way the coverage floors and the debt budget work. When a grandfathered
hotspot gets tests, the gate says so; remove it from `allow`.

The two most obvious first refactorings the gate points at: the worst function
in `openmeteo.go` (17) and in `app.go` (15), and coverage for
`http/server.go` and `app/app.go`, which would empty the hotspot allowlist.
