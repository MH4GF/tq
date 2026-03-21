# gh-notifications plugin

GitHub notification watcher and classifier for tq.

## Commands

### `/gh-notifications:watch`

Fetches GitHub notifications, classifies them by type and priority, and creates tq actions for each actionable notification.

- Automatically skips merged/closed PRs, Discussions, and Releases
- Detects remote action PRs (branches matching `tq-<id>-*`) and marks them done
- Selects appropriate prompt based on PR state (review-pr, fix-ci, respond-review, etc.)
- Matches notifications to existing tq tasks by URL or title keywords

## Usage

Triggered automatically via `tq schedule` or invoke manually:

```
/gh-notifications:watch
```

## Installation

Add to `~/.claude/settings.json`:

```json
{
  "enabledPlugins": {
    "gh-notifications@tq-marketplace": true
  }
}
```
