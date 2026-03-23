---
description: Create a tq action (auto-infer prompt or let user choose)
argument-hint: "<instruction>"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL: DO NOT execute the user's instruction yourself. Your ONLY job is to create a pending action that delegates the work to a queue worker.**

IMPORTANT: Run `tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Find task_id

Infer from session context. Use `tq search "<keyword>"` to find matching tasks by keyword.
If no matching task exists, create one with `tq task create`.

### 2. Build instruction

Consult `tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).

### 3. Create

```bash
tq action create --instruction '<instruction>' --title '<title>' --task <task_id>
```
