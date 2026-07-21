#!/usr/bin/env bash
# openapi-mirror-check.sh — fail if the two OpenAPI spec copies have drifted.
#
# The embedded spec (internal/adapters/http/openapi.yaml) is the source of truth.
# The mirror (api/openapi/openapi.yaml) must be byte-identical.
# Task 15 keeps them in sync; this gate enforces the invariant.
#
# Usage: ./scripts/openapi-mirror-check.sh [project-root]
# Exit 0 = identical, exit 1 = drift detected.

set -euo pipefail

ROOT="${1:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SRC="$ROOT/internal/adapters/http/openapi.yaml"
MIRROR="$ROOT/api/openapi/openapi.yaml"

if [ ! -f "$SRC" ]; then
    echo "ERROR: source spec not found: $SRC" >&2
    exit 1
fi

if [ ! -f "$MIRROR" ]; then
    echo "ERROR: mirror spec not found: $MIRROR" >&2
    exit 1
fi

if diff -q "$SRC" "$MIRROR" > /dev/null 2>&1; then
    echo "openapi-mirror-check: OK (files are byte-identical)"
    exit 0
else
    echo "openapi-mirror-check: FAILED — OpenAPI specs have drifted!" >&2
    echo "" >&2
    echo "  Source: $SRC" >&2
    echo "  Mirror: $MIRROR" >&2
    echo "" >&2
    echo "Fix: cp $SRC $MIRROR" >&2
    diff "$SRC" "$MIRROR" >&2 || true
    exit 1
fi
