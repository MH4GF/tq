---
description: openタスクの棚卸し。状況確認→整理提案→実行
argument-hint: "[project_name]"
---

# tq triage

open タスクを棚卸しして整理する。

## 手順

### 1. 収集

`tq task list --status open` と `tq action list` で open タスクとアクションを取得する。`$ARGUMENTS` があれば `--project` でフィルタする。

### 2. サマリー

プロジェクト別にテーブルで提示する:

| ID | タイトル | 経過 | 最新アクション |
|---|---|---|---|
| 42 | 機能Aの実装 | 3日 | #301 implement done — 実装完了・テスト通過 |
| 55 | バグBの修正 | 5日 | アクションなし |

running アクションは tmux pane の生存を確認し、stale かどうか判断する。

### 3. 提案

AskUserQuestion で整理アクションを提案する。以下の観点で判断する:

- **完了判定**: 最新アクションが成功 → タスクの要件を満たしているか確認。満たしていれば done に、不足があれば追加 action を作成
- **未着手**: アクションなし → まだやるか確認し、やるなら action 作成、やらないなら archived
- **停滞**: 失敗が続いている・長期間動きなし → アプローチ変更して再試行するか、archived にするか

提案はユーザー承認後に実行する。
