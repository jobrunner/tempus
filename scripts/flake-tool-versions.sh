#!/usr/bin/env bash
# flake-tool-versions.sh — print "<tool>\t<version>" for every package of the
# dev shell (flake.nix), as locked in flake.lock. flake-update.yml runs it
# before and after `nix flake update` to show in the PR which tools move.
#
# The list is read from the shell itself, so it cannot drift from flake.nix.
# Only evaluated, nothing is built. go is the bootstrap Go from nixpkgs; the Go
# that builds and tests follows go.mod's toolchain directive.
set -euo pipefail

sys=$(nix eval --impure --raw --expr builtins.currentSystem)
nix eval --json ".#devShells.${sys}.default.nativeBuildInputs" \
  --apply 'map (p: { name = p.pname or p.name; version = p.version or ""; })' \
  | jq -r '.[] | "\(.name)\t\(.version)"'
