# tq Best Practices

Operational guidance for getting good results out of tq. Each section is
self-contained; link to a specific section rather than copying its content
into skills or prompts.

## Dispatch mode selection

**Default to `interactive` or `experimental_bg`. Avoid `noninteractive`
unless an action genuinely requires headless `claude -p`.**

The reason is billing, not capability. `noninteractive` dispatches via
`claude -p` (`dispatch/noninteractive.go`). On Claude subscription plans,
non-interactive `claude -p` / Agent SDK usage draws from a **separate,
capped, non-rolling monthly Agent SDK credit** (per Anthropic's
[Use the Claude Agent SDK with your Claude plan](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan);
at time of writing: effective 2026-06-15, Pro $20 / Max 5x $100 /
Max 20x $200 — see the article for current figures). Once that credit
is exhausted,
further `claude -p` calls either stop or spill over to **pay-as-you-go
API rates** if extra usage is enabled. Interactive Claude Code in
terminals/IDEs — and `claude --bg`, which is a daemon-backed agent-view
session, not `claude -p` — consume the standard subscription usage
allowance instead and are *not* drawn from that credit. So a fleet of
high-frequency unattended `noninteractive` schedules is the case most
likely to burn the Agent SDK credit and tip into API billing;
`interactive`/`experimental_bg` do not.

### The four modes

| Mode | Launches | Surfaces in | Use when |
| --- | --- | --- | --- |
| `interactive` (default) | `claude` in a tmux window | tmux | A human watches/drives the session from a terminal. |
| `experimental_bg` | `claude --bg` | `claude agents` | Interactive-equivalent work without a tmux pane. `AskUserQuestion` / `ExitPlanMode` / `Edit` approvals are answered from the agents view, so plan-mode and approval-heavy work are fine here. **This is the recommended default for automated/scheduled actions.** |
| `noninteractive` | `claude -p --output-format json` | — (headless) | Only when the action *must* be headless: cloud/cross-event batch analysis, or pipelines that consume the structured `--output-format json` result. Accepts that this consumes the Agent SDK credit / API billing. |
| `remote` | `claude --remote` | Claude Code on the web | Cloud-executed actions (e.g. Cloud Routines). |

`experimental_bg` and `interactive` share the `MaxInteractive` slot pool
(both appear in `claude agents` as user-interactive sessions);
`noninteractive` has its own `MaxNonInteractive` pool. See
[dispatch.md](dispatch.md) for the concurrency model, slot caps, and
lifecycle mechanics, and [cli-reference.md](cli-reference.md) for the
`--meta` schema (`mode`, `claude_args`, `executor`).

### The permission-denial trade-off

`noninteractive` is the only mode that captures structured
`permission_denials` into action metadata (`dispatch/execute.go`): when
`claude -p` auto-denies a tool prompt it has no human to ask, so the
denial is recorded. Moving an action off `noninteractive` loses that
per-event signal. This is an acceptable trade in almost all cases:
cross-event improvement loops (session-log / hook-log analysis) do not
depend on it — they read telemetry produced by every mode — and
per-event auto-remediation tends to accrete defensive deny rules without
a correctness check. Prefer cross-event aggregation over the per-event
denial stream when deciding whether the loss matters.

### Decision rule

1. Does the action need to be headless (cloud batch, JSON-result
   pipeline)? → `remote` for cloud, `noninteractive` only if `claude -p`
   output is actually consumed.
2. Otherwise → `experimental_bg` (automated/scheduled) or `interactive`
   (a human is at the terminal).

Set the mode via `--meta`. `--meta` merges, so always specify `mode`
*and* `claude_args` together when changing an existing schedule/action,
otherwise stale values from the previous mode are silently kept:

```bash
tq schedule update <id> --meta '{"mode":"experimental_bg","claude_args":["--model","sonnet","--effort","low"]}'
```
