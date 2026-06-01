# tq Best Practices

Operational guidance for getting good results out of tq. Each section is
self-contained; link to a specific section rather than copying its content
into skills or prompts.

## Dispatch mode selection

All local actions launch as `claude --bg` background sessions and show
up in `claude agents`. See the official [Agent View
docs](https://code.claude.com/docs/en/agent-view#manage-multiple-agents-with-agent-view)
for the underlying mechanic. Because no mode runs `claude -p` anymore,
Pro / Max subscribers consume only their normal Claude Code allowance —
the capped Agent SDK credit no longer enters the picture.

The remaining `mode` choice picks a slot pool, not a launch path.

### The three modes

| Mode | Launches | Slot pool | Use when |
| --- | --- | --- | --- |
| `interactive` (default) | `claude --bg` | `MaxInteractive` (default 3) | Default for any locally executed action. The cap protects against piling on too many simultaneous user-facing sessions in `claude agents`. |
| `noninteractive` | `claude --bg` | `MaxNonInteractive` (default 5) | High-volume batch / scheduled work that should not crowd interactive sessions out of their slot pool. The launch path is identical to `interactive`; only the slot accounting differs. |
| `remote` | `claude --remote` | none (fire-and-forget) | Cloud-executed actions (e.g. Cloud Routines). |

See [dispatch.md](dispatch.md) for the concurrency model, slot caps, and
lifecycle mechanics, and [cli-reference.md](cli-reference.md) for the
`--meta` schema (`mode`, `claude_args`, `executor`).

### Decision rule

1. Does the action need to run in the cloud? → `remote`.
2. Otherwise → `interactive` (default) for human-facing or
   moderate-frequency work, `noninteractive` only when a long batch fleet
   would otherwise saturate the interactive slot pool.

Set the mode via `--meta`. `--meta` merges, so always specify `mode`
*and* `claude_args` together when changing an existing schedule or
action, otherwise stale values from the previous mode are silently kept.

```bash
tq schedule update <id> --meta '{"mode":"noninteractive","claude_args":["--model","sonnet","--effort","low"]}'
```
