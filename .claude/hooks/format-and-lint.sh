#!/usr/bin/env bash
# PostToolUse hook: format and lint Go files after Edit/Write.
# Advisory only — always exits 0.
set -euo pipefail

INPUT=$(cat)
FILE=$(echo "$INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null || true)

[[ "$FILE" == *.go ]] || exit 0

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$PROJECT_DIR"

# Format
gofmt -w "$FILE" 2>/dev/null || true
command -v goimports >/dev/null 2>&1 && goimports -w -local github.com/jobrunner/tempus "$FILE" 2>/dev/null || true

# Focused lint on the changed package
DIR=$(dirname "$FILE")
golangci-lint run --fix --timeout=60s "$DIR/..." 2>/dev/null || true

exit 0
