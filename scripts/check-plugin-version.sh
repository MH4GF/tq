#!/bin/bash
# Check that plugin.json version is updated when .claude-plugins/ files change

set -e

# Get staged files in .claude-plugins/
staged_plugin_files=$(git diff --cached --name-only --diff-filter=ACMR | grep '^\.claude-plugins/' || true)

if [ -z "$staged_plugin_files" ]; then
  # No plugin files changed, nothing to check
  exit 0
fi

# Find unique plugin directories that have changes
plugin_dirs=$(echo "$staged_plugin_files" | while read -r file; do
  # Extract plugin directory: .claude-plugins/{plugin-name}/
  echo "$file" | sed -E 's|^(\.claude-plugins/[^/]+)/.*|\1|'
done | sort -u)

exit_code=0

for plugin_dir in $plugin_dirs; do
  plugin_json="$plugin_dir/.claude-plugin/plugin.json"

  if [ ! -f "$plugin_json" ]; then
    echo "Warning: $plugin_json not found, skipping version check"
    continue
  fi

  # Check if plugin.json version field was changed in this commit
  version_changed=$(git diff --cached "$plugin_json" 2>/dev/null | grep -E '^\+.*"version"' || true)

  if [ -z "$version_changed" ]; then
    echo "Error: Files in $plugin_dir/ were modified but version in $plugin_json was not updated."
    echo ""
    echo "   Changed files:"
    echo "$staged_plugin_files" | grep "^$plugin_dir/" | sed 's/^/     - /'
    echo ""
    echo "   Please update the version in $plugin_json before committing."
    exit_code=1
  fi
done

if [ $exit_code -eq 0 ]; then
  echo "Plugin version check passed"
fi

exit $exit_code
