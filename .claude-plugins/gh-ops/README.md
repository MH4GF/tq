# gh-ops plugin

GitHub notification watcher and classifier for tq.

## Commands

### `/gh-ops:watch`

Watch GitHub notifications, classify them, and create tq actions.

- Fetches details per subject type: PRs via `gh pr view`, Issues via `gh issue view`, Releases via `gh release view`, Discussions via `gh api`
- Skips review requests already handled (own review submitted, or a team request assigned to someone else)
- Detects remote action PRs (branches matching `tq-<id>-*`) and marks the source action done
- Selects a slash-command instruction by PR state (review-pr / fix-conflict / fix-ci / respond-review / merge-pr / self-review), or falls back to a free-text instruction when no condition matches
- Matches notifications to existing tq tasks by URL or title keywords; creates new tasks when no match is found
- Marks each processed notification as read

### PR Processing Commands

The following commands are created as tq action instructions by `watch`, and can also be invoked manually:

| Command | Description |
|---|---|
| `/gh-ops:review-pr <PR_URL>` | Review another person's PR |
| `/gh-ops:fix-conflict <PR_URL>` | Resolve merge conflicts on a PR |
| `/gh-ops:fix-ci <PR_URL>` | Fix CI failures on a PR |
| `/gh-ops:respond-review <PR_URL>` | Respond to review comments on a PR |
| `/gh-ops:merge-pr <PR_URL>` | Judge and execute PR merge |
| `/gh-ops:self-review <PR_URL>` | Self-review a PR before requesting review |

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
