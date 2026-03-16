---
description: Create a tq action (auto-infer prompt or let user choose)
argument-hint: "<prompt or instruction>"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL: DO NOT execute the user's instruction yourself. Your ONLY job is to create a pending action that delegates the work to a queue worker.**

IMPORTANT: Run `tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Choose prompt

Run `tq prompt list`, then infer the best prompt from `$ARGUMENTS` and session context.
If unsure, present options to the user.

### 2. Find task_id

Infer from session context and search `tq task list --status open`.
If no matching task exists, create one with `tq task create`.

### 3. Build metadata

Consult `tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).
For prompts that rely on task-level data (e.g. `{{.Task.URL}}`), metadata can be `{}`.

### 4. Create

`tq action create <prompt> --title '<title>' --task <task_id> --meta '<json>'`
