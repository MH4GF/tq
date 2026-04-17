---
description: Create a tq action (auto-infer instruction or let user specify)
argument-hint: "<instruction>"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL: DO NOT execute the user's instruction yourself. Your ONLY job is to create a pending action that delegates the work to a queue worker.**

IMPORTANT: Run !`tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Find task_id

1. Identify the target project: check `tq project list` and match by work_dir or repo context.
2. Search within that project: `tq search "<keyword>" --project <project_id>`.
3. If a matching task exists, use its task_id. Otherwise create one with `tq task create --project <project_id>`.

### 2. Build instruction

Consult !`tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).

### 3. Create

```bash
tq action create '<instruction>' --title '<title>' --task <task_id>
```
