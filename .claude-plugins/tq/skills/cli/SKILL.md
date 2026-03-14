---
description: tq CLIによるタスク・アクション管理。「タスク作って」「アクション追加して」「完了にして」「状況見せて」で発動
allowed-tools: Bash(tq *), Bash(sqlite3 *), Bash(cat /tmp/tq-meta-*), Write(/tmp/tq-meta-*)
---

# tq CLI リファレンス

## タスク管理

```bash
tq task list
tq task create "タイトル" --project <project_id> --url "https://..."
tq task update <task_id> --status <open|review|done|blocked|archived>
```

- `--url` は任意。GitHub PR / Linear issue 等があれば付与する
- GitHub PR の URL からタイトルを取得する場合: `gh pr view <URL> --json title --jq '.title'`

## プロジェクト管理

```bash
tq project list
tq project create <NAME> <WORK_DIR> --meta '<json>'
tq project update <ID> --dispatch-enabled true/false   # ID は tq project list で確認
tq project delete <ID>
```

## アクション管理

```bash
tq action list
tq action list --task <task_id>
tq action list --status <pending|running|done|failed|cancelled>
tq action create <prompt_id> --task <task_id> --meta '<json>'
tq action done <action_id> '<result>'
tq action cancel <action_id> '<reason>'  # pending/running/failed → cancelled
tq action reset <action_id>      # failed/running/cancelled → pending
```

### プロンプト一覧

```bash
tq prompt list    # 利用可能なプロンプトを表示
```

## ディスパッチ（割り込み実行）

キューの順番を無視して特定アクションを即座に実行する。割り込みタスクに使用。

```bash
tq dispatch <action_id>           # 指定アクションを即座にディスパッチ
tq dispatch                       # 次のpendingアクションをディスパッチ
tq dispatch --session <name>      # tmuxセッションを指定（デフォルト: main）
```

## イベント履歴

タスクやアクションの状態変更経緯を追跡するときに使う。
「なぜこのタスクがarchivedされた？」「このアクションの状態遷移は？」といった調査に有用。

```bash
tq event list                                          # 直近50件
tq event list --entity action --id <action_id>         # 特定アクションの全履歴
tq event list --entity task --id <task_id>              # 特定タスクの全履歴
tq event list --limit 100                              # 件数指定
```

- タスクのステータス変更理由は `task.status_changed` イベントの `reason` フィールドに記録される
- アクションの完了結果は `action.status_changed` イベントの `result` フィールドで確認できる

