---
description: Mark a tq action as done and report results
argument-hint: "<action_id> [summary]"
allowed-tools: Bash(tq *)
---

# tq action done

IMPORTANT: Run `tq action done --help` first to understand result format guidance.

## Find action_id

1. Environment variable `TQ_ACTION_ID` — always check this first. It is pre-set by the dispatcher and avoids an extra CLI call.
2. Search running actions: `tq action list --status running`
3. If neither works, ask the user

## Execute

`tq action done <action_id> '<result>'`
