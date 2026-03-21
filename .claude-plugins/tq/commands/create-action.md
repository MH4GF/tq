---
description: Create a tq action (auto-infer prompt or let user choose)
argument-hint: "<prompt or instruction>"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL: DO NOT execute the user's instruction yourself. Your ONLY job is to create a pending action that delegates the work to a queue worker.**

IMPORTANT: Run `tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Decide: prompt template or direct instruction?

- If the task maps to an existing prompt template, use `--prompt`.
- If the user provides a slash command or direct instruction (e.g., "/github-pr review this"), use `--instruction` without a prompt.
- Both can be combined: `--prompt` for the template, `--instruction` for the specific instruction text.

Run `tq prompt list` if unsure which prompt to use.

### 2. Find task_id

Infer from session context and search `tq task list --status open`.
If no matching task exists, create one with `tq task create`.

### 3. Build metadata

Consult `tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).
For prompts that rely on task-level data (e.g. `{{.Task.URL}}`), metadata can be `{}`.

### 4. Create

```bash
# With prompt template
tq action create --prompt <prompt> --title '<title>' --task <task_id> --meta '<json>'

# With direct instruction (no template needed)
tq action create --instruction '<instruction>' --title '<title>' --task <task_id>

# Both (instruction available in template as {{index .Action.Meta "instruction"}})
tq action create --prompt <prompt> --instruction '<instruction>' --title '<title>' --task <task_id>
```
