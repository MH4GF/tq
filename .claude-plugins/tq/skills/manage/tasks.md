# タスク管理リファレンス

## コマンド

```bash
# タスク一覧
tq task list

# タスク作成
tq task create "タイトル" --project <project_name> --url "https://..."

# ステータス変更
tq task update <task_id> --status <open|review|done|blocked|archived>

# プロジェクト一覧（project_name の確認用）
tq project list

# プロジェクト作成
tq project create <NAME> <WORK_DIR> --metadata '<json>'

# プロジェクト編集（ID は tq project list で確認）
tq project edit <ID> --dispatch-enabled true/false

# プロジェクト削除
tq project delete <ID>
```

## プロジェクト管理の注意点

- `tq project edit` は ID（名前ではない）を指定する
- dispatch の有効/無効切替: `--dispatch-enabled true` / `--dispatch-enabled false`
  - `update` や `--dispatch` は存在しない。必ず `edit` と `--dispatch-enabled` を使う

## 運用ルール

- `--project` はプロジェクト名（ID ではない）を指定する
- `--url` は任意。GitHub PR / Linear issue / Notion ページ等があれば付与する
- GitHub PR の URL からタイトルを取得する場合: `gh pr view <URL> --json title --jq '.title'`
