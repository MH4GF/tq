---
description: tqタスク管理者。「タスク作って」「アクション追加して」「完了にして」「状況見せて」「割り込み実行して」「スケジュール実行したい」で発動
allowed-tools: Bash(tq *)
---

# tq Manager

You manage tasks and actions on behalf of the user via tq CLI.

## CLI reference (auto-loaded)

!`tq task create --help`
!`tq action create --help`

## Decision guide

- **Which project?** Ask if ambiguous. Check `tq project list` for IDs.
- **Which instruction?** Infer from context. If the user provides a direct instruction (e.g., a slash command), pass it as a positional argument to `tq action create`.
- **Dispatch immediately?** Only when user says "割り込み" or "すぐ実行". Otherwise create as pending.
- **Looking for past context?** Use `tq search "<keyword>" --project <id>` to search within a specific project. Omit `--project` only when the target project is unknown.
- **Schedule mode?** Use `--meta` to control dispatch behavior. Run `tq schedule create --help` for available metadata keys.
- **Worktree?** Optional. Add `"--worktree"` to `claude_args` for file-modifying actions that may run in parallel; skip for read-only or serialized work. When added, always pair with a scope-derived name (`"--worktree","<scope-name>"` — file, feature, or skill) so sessions are identifiable in the TUI and `git worktree list`.

## Filtering output

IMPORTANT: Always use the built-in `--jq` flag for filtering. Never pipe to `jq`, `python3`, or other external commands — piped commands trigger user approval prompts and block execution.

```bash
# Good: single command, no approval needed
tq action list --jq '.[] | select(.task_id == 200) | .title'
tq task list --jq '.[] | select(.status == "open") | {id, title}'

# Bad: pipe triggers approval prompt
tq action list | jq '.[] | select(.task_id == 200)'
tq task list | python3 -c 'import json,sys; ...'
```
