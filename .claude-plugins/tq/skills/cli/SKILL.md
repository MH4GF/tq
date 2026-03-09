---
description: tq CLIによるタスク・アクション管理。「タスク作って」「アクション追加して」「完了にして」「状況見せて」で発動
allowed-tools: Bash(tq *), Bash(sqlite3 *), Bash(cat /tmp/tq-meta-*), Write(/tmp/tq-meta-*)
---

# tq CLI リファレンス

## タスク管理

```bash
tq task list
tq task create "タイトル" --project <project_name> --url "https://..."
tq task update <task_id> --status <open|review|done|blocked|archived>
```

- `--project` はプロジェクト名（ID ではない）を指定する
- `--url` は任意。GitHub PR / Linear issue 等があれば付与する
- GitHub PR の URL からタイトルを取得する場合: `gh pr view <URL> --json title --jq '.title'`

## プロジェクト管理

```bash
tq project list
tq project create <NAME> <WORK_DIR> --metadata '<json>'
tq project edit <ID> --dispatch-enabled true/false   # ID は tq project list で確認
tq project delete <ID>
```

- `tq project edit` は ID（名前ではない）を指定する
- `update` や `--dispatch` は存在しない。必ず `edit` と `--dispatch-enabled` を使う

## アクション管理

```bash
tq action list
tq action list --task <task_id>
tq action list --status <pending|running|done|failed|waiting_human>
tq action create <prompt_id> --task <task_id> --meta '<json>' --source human
tq action done <action_id> '<result>'
tq action approve <action_id>    # waiting_human → pending
tq action reject <action_id>     # waiting_human → failed
tq action reset <action_id>      # failed/waiting_human/running → pending
```

### プロンプト一覧

```bash
tq prompt list    # 利用可能なプロンプトを表示
```

