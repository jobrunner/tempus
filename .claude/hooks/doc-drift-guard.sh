#!/usr/bin/env bash
# PreToolUse hook: block gh pr create if doc drift detected.
# Exit 2 to block, 0 to allow.
set -euo pipefail

INPUT=$(cat)
TOOL=$(echo "$INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || true)
CMD=$(echo "$INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('command',''))" 2>/dev/null || true)

[ "$TOOL" = "Bash" ] || exit 0
echo "$CMD" | grep -qE 'gh[[:space:]]+pr[[:space:]]+create' || exit 0

# Skip with SKIP_DOC_DRIFT=1
[ "${SKIP_DOC_DRIFT:-0}" = "1" ] && exit 0

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$PROJECT_DIR"

if OUT=$(./scripts/openapi-mirror-check.sh 2>&1); then
  exit 0
fi

echo "doc drift detected — not opening the PR (set SKIP_DOC_DRIFT=1 to override)"
echo "$OUT" >&2
exit 2
