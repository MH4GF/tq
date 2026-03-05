# ビュー・状況確認リファレンス

## コマンド

```bash
# タスク一覧
tq task list

# アクション一覧
tq action list

# TUI を起動（タスク・アクションをインタラクティブに確認）
tq ui

# プロジェクト一覧
tq project list
```

## DB 直接クエリ

tq CLI で取得できない情報は sqlite3 で直接取得する。DB パスは `~/.config/tq/tq.db`。

```bash
# 特定プロジェクトのアクティブタスク
sqlite3 ~/.config/tq/tq.db "SELECT t.id, t.status, t.title FROM tasks t JOIN projects p ON t.project_id = p.id WHERE p.name = '<project>' AND t.status NOT IN ('done', 'archived');"

# 最近の classify-next-action 結果
sqlite3 ~/.config/tq/tq.db "SELECT id, task_id, status, substr(result, 1, 500) FROM actions WHERE template_id = 'classify-next-action' ORDER BY id DESC LIMIT 10;"

# アクション結果の全文取得
sqlite3 ~/.config/tq/tq.db "SELECT result FROM actions WHERE id = <action_id>;"
```
