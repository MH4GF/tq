# gh-ops plugin

GitHub notification watcher and classifier for tq.

## Commands

### `/gh-ops:watch`

Fetches GitHub notifications, classifies them by type and priority, and creates tq actions for each actionable notification.

- Automatically skips merged/closed PRs, Discussions, and Releases
- Detects remote action PRs (branches matching `tq-<id>-*`) and marks them done
- Selects appropriate instruction based on PR state and creates tq actions with slash commands
- Matches notifications to existing tq tasks by URL or title keywords; creates new tasks when no match is found
- Marks each processed notification as read

### PR Processing Commands

The following commands are created as tq action instructions by `watch`, and can also be invoked manually:

| Command | Description |
|---|---|
| `/gh-ops:review-pr <PR_URL>` | Review another person's PR |
| `/gh-ops:fix-conflict <PR_URL>` | Resolve merge conflicts |
| `/gh-ops:fix-ci <PR_URL>` | Fix CI failures |
| `/gh-ops:respond-review <PR_URL>` | Respond to review comments |
| `/gh-ops:merge-pr <PR_URL>` | Judge and execute PR merge |
| `/gh-ops:self-review <PR_URL>` | Self-review before requesting review |

## Usage

Invoke manually:

```text
/gh-ops:watch
```

## Installation

Add to `~/.claude/settings.json`:

```json
{
  "enabledPlugins": {
    "gh-ops@tq-marketplace": true
  }
}
```
