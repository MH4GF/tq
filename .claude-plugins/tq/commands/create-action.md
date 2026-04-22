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

Infer from session context. Use `tq search "<keyword>"` to find matching tasks by keyword.
If no matching task exists, create one with `tq task create`.

### 2. Build instruction

Consult !`tq action create --help` for instruction format guidance.
Use only information from the current session — do not investigate files (that is the worker's job).

### 3. Create

```bash
tq action create '<instruction>' --title '<title>' --task <task_id>
```
