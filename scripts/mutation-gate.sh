#!/usr/bin/env bash
#
# mutation-gate.sh — per-package mutation thresholds (RATCHET, raise-only).
# The single entrypoint: `make mutation` and the CI workflow both call only
# this script, so local and CI cannot diverge. Thresholds live in
# .mutation-thresholds.
#
# Usage: mutation-gate.sh [package ...]     (default: every package in the module)
#
# Runs gremlins once per package — NEVER with a `...` wildcard, which gremlins
# does not expand: it silently generates zero mutants and exits 0. A real service
# shipped a gate built on `gremlins unleash ./internal/...` and it measured
# nothing at all through eight tasks while reporting success on every PR.
#
# Fails when a package is below its threshold, is missing from
# .mutation-thresholds, or produces no mutants without declaring NONE.
set -euo pipefail

# LC_ALL=C forces a dot decimal separator so awk's numeric compares work in
# comma-locale environments (e.g. de_DE would stringify "98,15").
export LC_ALL=C

THRESHOLDS="${THRESHOLDS:-.mutation-thresholds}"
GREMLINS="${GREMLINS:-gremlins}"
MODULE="$(go list -m)"

[ -f "$THRESHOLDS" ] || { echo "mutation-gate: thresholds not found: $THRESHOLDS" >&2; exit 2; }
command -v "$GREMLINS" >/dev/null 2>&1 || { echo "mutation-gate: $GREMLINS not on PATH" >&2; exit 2; }

# Package list: the arguments, or every package in the module. Read with a plain
# `while read` loop, not mapfile: mapfile is a bash 4 builtin and the bash macOS
# ships is 3.2, so it would abort this script on the very platform the local run
# advertises.
if [ "$#" -gt 0 ]; then
  PACKAGES=("$@")
else
  # `go list` runs into a FILE, not a process substitution: inside `< <(...)`
  # its exit status is lost, so a failed discovery would yield an empty package
  # list, skip the loop entirely and report OK — a gate measuring nothing.
  #
  # -f skips packages with no production Go files (a test-only package such as
  # internal/arch generates no mutants and would otherwise have to be declared
  # NONE just to keep the gate quiet).
  LIST="$(mktemp)"
  trap 'rm -f "$LIST"' EXIT
  if ! go list -f '{{if .GoFiles}}{{.ImportPath}}{{end}}' ./... > "$LIST"; then
    echo "mutation-gate: go list failed — refusing to run a gate over an unknown package set" >&2
    exit 2
  fi
  PACKAGES=()
  while IFS= read -r p; do
    [ -n "$p" ] && PACKAGES+=("$p")
  done < <(sed "s|^${MODULE}/||;s|^${MODULE}\$|.|" "$LIST")
  if [ "${#PACKAGES[@]}" -eq 0 ]; then
    echo "mutation-gate: no packages discovered — the gate would be vacuous" >&2
    exit 2
  fi
fi

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

lookup() { # lookup <pkg> -> "<efficacy> <mcover>" | "NONE" | ""
  grep -vE '^\s*#|^\s*$' "$THRESHOLDS" | awk -v p="$1" '$1 == p { $1 = ""; sub(/^ +/, ""); print; exit }'
}

fail=0
for pkg in "${PACKAGES[@]}"; do
  want="$(lookup "$pkg")"
  if [ -z "$want" ]; then
    printf "%-42s   ▼ NOT IN %s (add it, with measured numbers or NONE)\n" "$pkg" "$THRESHOLDS"
    fail=1; continue
  fi

  # Keep the status. gremlins' THRESHOLD verdict is unusable (v0.5.1 exits 0
  # below any threshold), which is why the numeric comparison below exists — but
  # a non-zero exit still distinguishes "ran and reported" from "crashed after
  # printing the banner", and the latter must not be parsed as a result.
  "$GREMLINS" unleash "./$pkg" >"$OUT" 2>&1
  grc=$?

  if [ "$grc" -ne 0 ] && grep -q "Mutation testing completed" "$OUT"; then
    printf "%-42s   ▼ gremlins exited %s after reporting — result not trusted\n" "$pkg" "$grc"
    sed 's/^/      /' "$OUT"
    fail=1; continue
  fi

  if ! grep -q "Mutation testing completed" "$OUT"; then
    # Either gremlins errored, or it generated nothing at all. "No results to
    # report." is the vacuous-gate signature and must never read as a pass.
    if grep -q "No results to report" "$OUT"; then
      if [ "$want" = "NONE" ]; then
        printf "%-42s   n/a   (no mutants, as declared)\n" "$pkg"; continue
      fi
      printf "%-42s   ▼ NO MUTANTS GENERATED — the gate would measure nothing here\n" "$pkg"
      fail=1; continue
    fi
    printf "%-42s   ▼ gremlins failed:\n" "$pkg"; sed 's/^/      /' "$OUT"
    fail=1; continue
  fi

  eff="$(awk '/^Test efficacy:/ { gsub(/%/, "", $3); print $3; exit }' "$OUT")"
  mcov="$(awk '/^Mutator coverage:/ { gsub(/%/, "", $3); print $3; exit }' "$OUT")"
  counts="$(awk '/^Killed:/ { print; exit }' "$OUT")"

  if [ "$want" = "NONE" ]; then
    printf "%-42s   ▼ DECLARED NONE but generated mutants (%s) — give it real thresholds\n" "$pkg" "$counts"
    fail=1; continue
  fi

  # A missing or non-numeric metric means the report format changed under us.
  # Never let that reach the comparison: awk would coerce "" and the package
  # would be judged against a value nobody measured.
  case "$eff" in ''|*[!0-9.]*) eff="" ;; esac
  case "$mcov" in ''|*[!0-9.]*) mcov="" ;; esac
  if [ -z "$eff" ] || [ -z "$mcov" ]; then
    printf "%-42s   ▼ could not read efficacy/mcover from the report — format changed?\n" "$pkg"
    sed 's/^/      /' "$OUT"
    fail=1; continue
  fi

  read -r mineff minmcov <<<"$want"
  status=""
  if awk -v v="$eff" -v m="$mineff" 'BEGIN{exit !(v < m)}'; then status="efficacy"; fi
  if awk -v v="$mcov" -v m="$minmcov" 'BEGIN{exit !(v < m)}'; then status="${status:+$status+}mcover"; fi

  if [ -n "$status" ]; then
    printf "%-42s eff %6s%%/%s%%  mcov %6s%%/%s%%  ▼ BELOW THRESHOLD (%s)\n" \
      "$pkg" "$eff" "$mineff" "$mcov" "$minmcov" "$status"
    grep -E "^\s+(LIVED|NOT COVERED)" "$OUT" | sed 's/^/      /' || true
    fail=1
  else
    printf "%-42s eff %6s%% >= %s%%   mcov %6s%% >= %s%%\n" "$pkg" "$eff" "$mineff" "$mcov" "$minmcov"
  fi
done

[ "$fail" -eq 0 ] && echo "mutation-gate: OK"
exit "$fail"
