#!/usr/bin/env bash
# PostToolUse hook: advisory debt-guard ratchet check.
# Advisory only — always exits 0.
set -euo pipefail

INPUT=$(cat)
FILE=$(echo "$INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null || true)

[[ "$FILE" == *.go ]] || exit 0

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$PROJECT_DIR"

./scripts/debt-guard.sh 2>&1 || true

exit 0
