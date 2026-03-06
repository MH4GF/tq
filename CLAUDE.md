# CLAUDE.md

## プロジェクト概要

tq (Task Queue) — タスクとアクションをSQLiteで管理し、Claude Codeワーカーで自動処理するタスクキューCLI/TUI。

## コマンド

- ビルド: `go build ./...`
- テスト: `go test ./...`
- インストール: `GOBIN="$HOME/go/bin" go install .`
- 単一テスト: `go test ./db/ -run TestTaskCreate`

## アーキテクチャ

```
main.go          → エントリポイント
cmd/             → cobra CLI コマンド定義
db/              → SQLite データ層（schema.sql, CRUD操作）
dispatch/        → アクションディスパッチ（interactive/noninteractive/ralph loop）
tui/             → Bubble Tea TUI（queue タブ, tasks タブ）
view/            → デイリーノート出力
source/github/   → GitHub通知・PR取得
prompt/          → プロンプトローダー（frontmatter + Go template）
testutil/        → テスト用ヘルパー（インメモリDB）
```

## データディレクトリ

- DB: `~/.config/tq/tq.db`（固定）
- プロンプト: `~/.config/tq/prompts/`

## スタイル

- テストは `*_test.go` でテーブル駆動テスト
- テストDBは `testutil.NewTestDB()` でインメモリSQLite
- エラーは `fmt.Errorf` でラップ
