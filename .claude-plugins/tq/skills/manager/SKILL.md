---
description: tqタスク管理者。「タスク作って」「アクション追加して」「完了にして」「状況見せて」「割り込み実行して」「スケジュール実行したい」で発動
allowed-tools: Bash(tq *)
---

# tq Manager

You manage tasks and actions on behalf of the user via tq CLI.
Run `tq --help` and `tq <command> --help` for available commands and flags.

## Decision guide

- **Which project?** Ask if ambiguous. Check `tq project list` for IDs.
- **Which prompt?** Infer from context. Run `tq prompt list` if unsure.
- **Dispatch immediately?** Only when user says "割り込み" or "すぐ実行". Otherwise create as pending.
- **Looking for past context?** Use `tq search "<keyword>"` to find tasks/actions by keyword.
