---
description: Create a tq action (auto-infer instruction or let user specify)
argument-hint: "<instruction>"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL:**

- **This command delegates work to a separate worker session.** Creating the pending action IS the deliverable — the user already chose delegation by invoking this command.
- **DO NOT execute the instruction yourself, and DO NOT ask whether to run it now** — always just create the action.
- **DO NOT dispatch the action** (e.g. `tq action dispatch`) unless explicitly asked. Pending is the correct final state; `tq ui` picks it up.
- **After creating the action, report the ID and stop.** No follow-up execution or offers.

IMPORTANT: Run !`tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Find task_id

1. Identify the target project: check `tq project list` and match by work_dir or repo context.
2. Search within that project: `tq search "<keyword>" --project <project_id>`.
3. If a matching task exists, use its task_id. Otherwise create one with `tq task create --project <project_id>`.

### 2. Build instruction

Consult !`tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).

If the instruction spans multiple lines (free-text context, embedded URLs followed by next-step bullets, etc.), set `--meta '{"mode":"noninteractive"}'`. The default `interactive` mode rejects instructions containing newlines or other C0 control bytes (except tab) because they would fragment the tmux `send-keys` shell command — see `docs/cli-reference.md` `tq action create` for the full constraint.

### 3. Create

```bash
tq action create '<instruction>' --title '<title>' --task <task_id>
```

### Worktree (optional)

Omit `--worktree` for read-only or serialized actions. Use it when the action modifies files and may run in parallel.

When set, always pair with a scope-derived name (target file, feature, bug ID, or skill) so sessions are identifiable in the TUI and `git worktree list`:

```bash
tq action create '<instruction>' --title '<title>' --task <task_id> \
  --meta '{"claude_args":["--worktree","fix-login-csrf"]}'
```
