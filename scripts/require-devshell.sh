# require-devshell.sh — sourced by the gate scripts; the Makefile has the same guard.
#
# In CI the tools come from the flake's dev shell or not at all: a runner image
# ships its own Go, node and python, and a step outside the shell would use
# them without a word. `nix develop` (and direnv's `use flake`) set
# IN_NIX_SHELL. Locally nothing is enforced; the shell is still recommended.
if [ -n "${CI:-}" ] && [ -z "${IN_NIX_SHELL:-}" ]; then
  echo "${0##*/}: CI runs this only in the flake's dev shell (.github/actions/nix-shell)" >&2
  exit 2
fi
