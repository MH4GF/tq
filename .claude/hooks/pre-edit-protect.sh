#!/bin/bash
# PreToolUse hook: block edits to protected config files

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

BASENAME=$(basename "$FILE_PATH")

case "$BASENAME" in
  .golangci.yml|lefthook.yml)
    echo "Editing $BASENAME is not allowed. This file is a protected configuration." >&2
    exit 2
    ;;
esac

exit 0
