#!/bin/bash
# PostToolUse hook: auto-format and lint .go files after Edit/Write

export PATH="$(go env GOPATH)/bin:$PATH"

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process .go files
if [[ "$FILE_PATH" != *.go ]]; then
  exit 0
fi

# Check file exists (Write may create new files)
if [[ ! -f "$FILE_PATH" ]]; then
  exit 0
fi

gofumpt -w "$FILE_PATH" 2>&1
golangci-lint run --fix "$FILE_PATH" 2>&1

exit 0
