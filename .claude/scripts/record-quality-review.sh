#!/bin/bash
set -euo pipefail

mkdir -p .claude/tmp
sha=$(git rev-parse HEAD)
ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
state_file=".claude/tmp/quality-review-state.json"
[ -f "$state_file" ] || echo '{"reviewed":[]}' > "$state_file"
jq --arg sha "$sha" --arg ts "$ts" \
  '.reviewed = ((.reviewed + [{"sha":$sha,"timestamp":$ts}]) | .[-50:])' \
  "$state_file" > "$state_file.tmp" && mv "$state_file.tmp" "$state_file"
