#!/usr/bin/env bash
#
# harness-check.sh — completeness of the quality harness
# (a FITNESS FUNCTION).
#
#
# WHY THIS EXISTS. The harness in this skill is a set of gates, and every one of
# them is easy to leave out at scaffolding time — nothing goes red when a file
# simply is not there. Measured in this service family: one service shipped
# without dependabot.yml and without any Actions-security scan, and sat on an
# outdated Go minor through three releases while four siblings were bumped
# automatically. Nobody was careless; there was just no gate.
#
# WHAT IT CHECKS. Existence AND wiring. Existence alone is not enough:
#   - a mutation.yml that calls `gremlins unleash ./internal/...` exists and
#     measures NOTHING (gremlins does not expand the wildcard: zero mutants,
#     exit 0, green check),
#   - a workflow with a hardcoded `go-version: '1.x'` exists and silently
#     ignores the toolchain bump Dependabot just wrote into go.mod.
#
# ESCAPE HATCH. .harness-waivers takes one line per deliberately omitted item
# with a reason, and a trailing count. The count is a RATCHET: it may only go
# down. Omitting is allowed; omitting silently is not.
#
# Exit: 0 green, 1 gap, 2 setup error.

set -uo pipefail
export LC_ALL=C

WAIVERS="${WAIVERS:-.harness-waivers}"
WORKFLOWS="${WORKFLOWS:-.github/workflows}"

fail=0
MISSING=""

[ -d "$WORKFLOWS" ] || { echo "harness-check: $WORKFLOWS not found — run me from the repo root" >&2; exit 2; }

waived() {
  [ -f "$WAIVERS" ] || return 1
  grep -qE "^[[:space:]]*$1([[:space:]]|#|\$)" "$WAIVERS"
}

# check <key> <description> <command...>
check() {
  key="$1"; desc="$2"; shift 2
  if "$@" >/dev/null 2>&1; then
    printf '  ok       %-24s %s\n' "$key" "$desc"
  elif waived "$key"; then
    printf '  waived   %-24s %s\n' "$key" "$desc"
  else
    printf '  MISSING  %-24s %s\n' "$key" "$desc"
    MISSING="$MISSING $key"
    fail=1
  fi
}

has_file() { [ -f "$1" ]; }
greps()    { grep -qE "$2" "$1" 2>/dev/null; }

# No workflow may pin a Go version. Dependabot's gomod ecosystem bumps the
# `toolchain` directive in go.mod; CI only follows if it reads from there.
#
# TWO forms, and the second is the one that hides. A literal
#   go-version: '1.25'
# is obvious. The indirect form
#   env:
#     GO_VERSION: '1.25'
#   ...
#     go-version: ${{ env.GO_VERSION }}
# reads like configuration-as-a-variable and is just as much a hardcoded pin —
# a real repo had nine workflows clean and one pinned this way, and an earlier
# version of this check reported all of them green.
no_hardcoded_go() {
  ! grep -rhE "^[[:space:]]*go-version:[[:space:]]*['\"]?[0-9]|^[[:space:]]*GO_VERSION:[[:space:]]*['\"]?[0-9]" \
      "$WORKFLOWS" 2>/dev/null | grep -q .
}

# gremlins must never be handed a `...` wildcard — it does not expand it and
# generates zero mutants while exiting 0.
#
# Comment lines are stripped first. Without that this check fires on the very
# scripts that DOCUMENT the trap (this file included), which is both wrong and
# the kind of false positive that teaches people to ignore the gate.
no_wildcard_unleash() {
  ! grep -rhE "unleash[^|&;]*\.\.\." "$WORKFLOWS" Makefile scripts 2>/dev/null \
    | grep -vE "^[[:space:]]*#" \
    | grep -q .
}

# An `on: release` workflow does not fire for a release-please release WHEN
# release-please runs under the default GITHUB_TOKEN (GitHub's recursion guard).
# It fires normally under a GitHub App token or a PAT — measured: a repo whose
# release-please uses actions/create-github-app-token has had its `on: release`
# image build succeed for every release.
#
# So this check only applies to the default-token case. Skip it when the release
# workflow mints an app token or uses a PAT, or it flags a working setup.
no_on_release() {
  if grep -rqE "create-github-app-token|secrets\.(RELEASE_|.*_PAT|GH_PAT)" \
       "$WORKFLOWS" 2>/dev/null; then
    return 0   # not the default token — `on: release` is fine here
  fi
  ! awk '
    /^on:[[:space:]]*$/       { inon=1; next }
    /^[^[:space:]#]/          { inon=0 }
    inon && /^[[:space:]]+release:[[:space:]]*$/ { found=1 }
    END { exit !found }
  ' "$WORKFLOWS"/*.yml 2>/dev/null
}

echo "harness-check: mandatory harness components"
echo
echo "— Supply chain"
check dependabot           "dependabot.yml"                  has_file .github/dependabot.yml
check dependabot-gomod     "gomod ecosystem"                 greps .github/dependabot.yml 'package-ecosystem:[[:space:]]*["'"'"']?gomod'
check dependabot-actions   "github-actions ecosystem"        greps .github/dependabot.yml 'package-ecosystem:[[:space:]]*["'"'"']?github-actions'
if [ -f Dockerfile ]; then
  check dependabot-docker  "docker ecosystem (Dockerfile)"   greps .github/dependabot.yml 'package-ecosystem:[[:space:]]*["'"'"']?docker'
fi
if [ -f .gitmodules ]; then
  check dependabot-submod  "gitsubmodule ecosystem"          greps .github/dependabot.yml 'package-ecosystem:[[:space:]]*["'"'"']?gitsubmodule'
fi
check dependabot-automerge "auto-merge workflow"             has_file "$WORKFLOWS/dependabot-auto-merge.yml"

echo "— Actions security"
check zizmor-config        "zizmor.yml policy"               has_file .github/zizmor.yml
check zizmor-workflow      "actions-security.yml"            has_file "$WORKFLOWS/actions-security.yml"
check zizmor-wired         "workflow uses the policy"        greps "$WORKFLOWS/actions-security.yml" 'config[= ]\.github/zizmor\.yml'
check vuln-scan            "scheduled vulnerability scan"    greps "$WORKFLOWS/vuln-scan.yml" 'govulncheck'

echo "— Ratchets"
check debt-budget          ".debt-budget"                    has_file .debt-budget
check debt-script          "scripts/debt-guard.sh"           has_file scripts/debt-guard.sh
check debt-wired           "debt-guard wired into verify"    greps Makefile '^verify:.*debt-guard' 
check coverage-floors      ".coverage-floors"                has_file .coverage-floors
check coverage-script      "scripts/coverage-gate.sh"        has_file scripts/coverage-gate.sh
check coverage-wired       "coverage gate called by make"    greps Makefile 'coverage-gate\.sh' 
check gremlins-config      ".gremlins.yaml"                  has_file .gremlins.yaml
check mutation-thresholds  ".mutation-thresholds"            has_file .mutation-thresholds
check mutation-script      "scripts/mutation-gate.sh"        has_file scripts/mutation-gate.sh
check mutation-wired       "mutation.yml uses the gate"      greps "$WORKFLOWS/mutation.yml" 'mutation-gate\.sh'
check mutation-vacuity     "no ... wildcard to unleash"      no_wildcard_unleash
check codecharta-baseline  ".codecharta-ratchet.json"        has_file .codecharta-ratchet.json
check codecharta-script    "scripts/codecharta-ratchet.py"   has_file scripts/codecharta-ratchet.py
check codecharta-workflow  "codecharta.yml"                  has_file "$WORKFLOWS/codecharta.yml"
check arch-test            "internal/arch/arch_test.go"      has_file internal/arch/arch_test.go
check arch-baseline        ".arch-baseline"                  has_file .arch-baseline
check arch-nocache         "arch test runs with -count=1"    greps Makefile '(go|\$\(GO\)) test[^#]*-count=1[^#]*internal/arch' 

echo "— Release and legal"
check commitlint-config    ".commitlintrc.yml"               has_file .commitlintrc.yml
check commitlint-workflow  "commitlint.yml"                  has_file "$WORKFLOWS/commitlint.yml"
check release-config       "release-please-config.json"      has_file release-please-config.json
check release-manifest     ".release-please-manifest.json"   has_file .release-please-manifest.json
check release-workflow     "release-please.yml"              has_file "$WORKFLOWS/release-please.yml"
check goreleaser           ".goreleaser.yml"                 has_file .goreleaser.yml
check no-on-release        "no on:release workflow"          no_on_release
check license              "LICENSE"                         has_file LICENSE

echo "— Toolchain hygiene"
check go-version-file      "no hardcoded go-version"         no_hardcoded_go

# Waiver ratchet.
echo
if [ -f "$WAIVERS" ]; then
  lines=$(grep -vE '^[[:space:]]*#|^[[:space:]]*$' "$WAIVERS")
  declared=$(printf '%s\n' "$lines" | tail -1 | tr -d '[:space:]')
  total=$(printf '%s\n' "$lines" | grep -c .)
  actual=$((total - 1))
  case "$declared" in
    ''|*[!0-9]*) echo "harness-check: $WAIVERS has no trailing count line" >&2; exit 2 ;;
  esac
  echo "waivers: $actual declared, baseline $declared"
  if [ "$actual" -gt "$declared" ]; then
    echo "FAIL: waivers grew past the baseline ($actual > $declared)"
    fail=1
  elif [ "$actual" -lt "$declared" ]; then
    echo "✓ waivers dropped — lower the baseline to $actual to lock it in"
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo
  echo "harness-check: FAILED — missing:$MISSING"
  echo
  echo "Either add the artefact (templates live in the new-go-service skill),"
  echo "or record it in $WAIVERS with a reason and raise the count."
  exit 1
fi

echo
echo "harness-check: OK"
