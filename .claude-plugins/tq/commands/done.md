---
description: Mark a tq action as done and report results
argument-hint: "<action_id> [summary]"
allowed-tools: Bash(tq *)
---

# tq action done

## Find action_id

1. Environment variable `TQ_ACTION_ID` — always check this first. It is pre-set by the dispatcher and avoids an extra CLI call.
2. Search running actions: `tq action list --status running`
3. If neither works, ask the user

## Execute

IMPORTANT: Run `tq action done --help` for the full result format guidance.

`tq action done <action_id> '<result>'`

Result must use structured sections: outcome, decisions, artifacts, remaining.
Do NOT describe process steps — session logs capture that.
