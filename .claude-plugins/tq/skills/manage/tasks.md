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
```

## 運用ルール

- `--project` はプロジェクト名（ID ではない）を指定する
- `--url` は任意。GitHub PR / Linear issue / Notion ページ等があれば付与する
- GitHub PR の URL からタイトルを取得する場合: `gh pr view <URL> --json title --jq '.title'`
