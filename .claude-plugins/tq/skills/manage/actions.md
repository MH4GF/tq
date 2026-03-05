# アクション管理リファレンス

## コマンド

```bash
# アクション一覧
tq action list
tq action list --task <task_id>
tq action list --status <pending|running|done|failed|waiting_human>

# アクション作成
tq action create <template_id> --task <task_id>
tq action create implement --task <task_id> --meta '<json>' --source human

# アクション完了
tq action done <action_id> '<result_json>'

# 承認（waiting_human → pending）
tq action approve <action_id>

# 却下（waiting_human → failed）
tq action reject <action_id>
```

## テンプレート一覧

| テンプレート | 用途 | interactive |
|-------------|------|-------------|
| `implement` | 汎用実装タスク | yes |
| `self-review` | PR セルフレビュー | yes |
| `respond-review` | レビューコメント対応 | yes |
| `fix-ci` | CI 失敗修正 | yes |
| `fix-conflict` | コンフリクト解消 | yes |
| `merge-pr` | PR マージ | yes |
| `classify` | 通知分類（内部用） | no |
| `classify-next-action` | 完了後の次アクション判断（内部用） | no |

## implement アクションの作成

`--meta` に instruction を含む JSON を渡す。複雑な内容は temp file 経由で渡す:

```bash
# 1. JSON をファイルに書き出し
Write /tmp/tq-meta-<task_id>.json with:
{"instruction":"目的: ...\n\n関連コンテキスト:\n- ...\n\n制約:\n- ..."}

# 2. ファイルから読み込んで作成
tq action create implement --task <task_id> --meta "$(cat /tmp/tq-meta-<task_id>.json)" --source human
```

## failed アクションのリセット

`approve` は `waiting_human` のみ対応。`failed` のリセットは直接 DB 操作:

```bash
sqlite3 ~/.config/tq/tq.db "UPDATE actions SET status = 'pending', result = '' WHERE id = <action_id>;"
```

## アクション結果の確認

result が長い場合は DB から直接取得:

```bash
sqlite3 ~/.config/tq/tq.db "SELECT result FROM actions WHERE id = <action_id>;"
```
