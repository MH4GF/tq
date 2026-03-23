# gh-notifications plugin

GitHub notification watcher and classifier for tq.

## Commands

### `/gh-notifications:watch`

Fetches GitHub notifications, classifies them by type and priority, and creates tq actions for each actionable notification.

- Automatically skips merged/closed PRs, Discussions, and Releases
- Detects remote action PRs (branches matching `tq-<id>-*`) and marks them done
- Selects appropriate instruction based on PR state and creates tq actions with slash commands
- Matches notifications to existing tq tasks by URL or title keywords

### PR Processing Commands

The following commands are created as tq action instructions by `watch`, and can also be invoked manually:

| Command | Description |
|---|---|
| `/gh-notifications:review-pr <PR_URL>` | Review another person's PR |
| `/gh-notifications:fix-conflict <PR_URL>` | Resolve merge conflicts |
| `/gh-notifications:fix-ci <PR_URL>` | Fix CI failures |
| `/gh-notifications:respond-review <PR_URL>` | Respond to review comments |
| `/gh-notifications:merge-pr <PR_URL>` | Judge and execute PR merge |
| `/gh-notifications:self-review <PR_URL>` | Self-review before requesting review |

## Usage

Triggered automatically via `tq schedule` or invoke manually:

```text
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
