#!/usr/bin/env bash
#
# check-workflow-jobs.sh — every CI job runs in the flake's dev shell, unless
# .github/nix-exempt-jobs lists it with a reason.
#
# A runner image ships its own Go, node, python, …, so a step outside the dev
# shell silently uses the image's versions. Deny by default, on the workflow
# STRUCTURE only: what a `run:` script says is never inspected, so no shell
# syntax can get around it. A job that is not exempt
#
#   * has `uses: ./.github/actions/nix-shell`, preceded by nothing but
#     actions/checkout,
#   * runs its steps in `nix develop …`: the job's or the workflow's
#     defaults.run.shell, and no step sets another shell,
#   * uses only actions that bring no toolchain of their own (ALLOWED below);
#     a setup-* or linter action would put its own binaries on PATH.
#
# Parsed with yq (a real YAML parser) and jq, both from the dev shell.
#
# Usage: check-workflow-jobs.sh [workflow dir] [exempt list]   Exit 0 ok, 1 violation.

set -euo pipefail

WORKFLOWS="${1:-.github/workflows}"
EXEMPT="${2:-.github/nix-exempt-jobs}"
NIX_ACTION="./.github/actions/nix-shell"
# Actions a dev-shell job may use besides NIX_ACTION. Extend only with
# actions that install no language runtime or tool.
ALLOWED='["actions/checkout","actions/upload-artifact","actions/create-github-app-token","github/codeql-action/upload-sarif"]'

# awk, not grep: an empty list must not end the script under pipefail.
exempt=$(awk '!/^[[:space:]]*(#|$)/ { print $1 }' "$EXEMPT" | jq -R . | jq -sc .)
jobs_seen=""
fail=0

# GitHub runs both extensions; nullglob drops the one that has no files.
shopt -s nullglob
for wf in "$WORKFLOWS"/*.yml "$WORKFLOWS"/*.yaml; do
  name=$(basename "$wf")
  out=$(yq -o=json '.' "$wf" | jq -r \
    --arg f "$name" --arg nix "$NIX_ACTION" \
    --argjson ex "$exempt" --argjson allowed "$ALLOWED" '
    (.defaults.run.shell // "") as $wf_shell
    | .jobs | to_entries[]
    | (.key) as $job | (.value) as $j | ($f + ":" + $job) as $id
    | ($j.steps // []) as $steps
    | "job \($id)",
      if ($ex | index($id)) != null then
        if any($steps[]; .uses == $nix) then "\($id): listed as exempt but runs in the dev shell; drop it from the list" else empty end
      else
        (if $j.uses then "\($id): calls a reusable workflow; it cannot be checked here" else empty end),
        ($steps | map(.uses == $nix) | index(true)) as $n
        | (if $n == null then "\($id): no \($nix) step"
           else $steps[:$n][]
             | select(.run != null or ((.uses // "") | startswith("actions/checkout@") | not))
             | "\($id): step \"\(.name // .uses // "run")\" comes before \($nix)"
           end),
          (($j.defaults.run.shell // $wf_shell) as $sh
           | if ($sh | startswith("nix develop")) then empty
             else "\($id): default shell \"\($sh)\" is not nix develop (set defaults.run.shell)" end),
          ($steps[] | select(.shell != null and (.shell | startswith("nix develop") | not))
           | "\($id): step \"\(.name // "run")\" runs in shell \"\(.shell)\""),
          ($steps[] | select(.uses != null and .uses != $nix) | (.uses | sub("@.*$"; "")) as $a
           | select(($allowed | index($a)) == null)
           | "\($id): uses \($a), which is not allowed in a dev-shell job")
      end')
  while IFS= read -r line; do
    case "$line" in
      "job "*) jobs_seen="$jobs_seen ${line#job } " ;;
      *) echo "$WORKFLOWS/$line"; fail=1 ;;
    esac
  done <<< "$out"
done

for id in $(jq -r '.[]' <<< "$exempt"); do
  case "$jobs_seen" in
    *" $id "*) ;;
    *) echo "$EXEMPT: $id does not exist; drop it from the list"; fail=1 ;;
  esac
done

exit "$fail"
