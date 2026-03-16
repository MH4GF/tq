# tq — Task Queue

タスクとアクションをSQLiteで管理し、Claude Codeワーカーで自動処理するタスクキュー。

## セットアップ

```bash
cd tq && go install .
```

DB は `~/.config/tq/tq.db` に配置（固定）。プロンプトは `~/.config/tq/prompts/` に配置。

### プロジェクト登録

初回セットアップ時にプロジェクトを登録する:

```bash
tq project create myapp ~/ghq/github.com/org/myapp --meta '{"gh_owner":"org","repos":["org/myapp"]}'
```

登録済みプロジェクトの確認:

```bash
tq project list
```

## 人間向け: TUI (`tq ui`)

人間はTUIで状況を監視する。TUIは読み取り専用で、書き込み操作は Claude Code に指示して CLI 経由で行う。

```bash
tq ui
```

バックグラウンドで以下が自動実行される:
- **Queue Worker**: pending アクションを検出して自動ディスパッチ。スケジュール（cron）も Queue Worker が管理し、時刻到達時にアクションを自動生成する

### 実行状況を確認する

- **Tasks タブ** (`1`): Project → Task → Actions のツリービュー
- **Schedules タブ** (`2`): スケジュール一覧。有効/無効の切替・削除

### キーバインド

| キー | Tasks タブ | Schedules タブ |
|------|-----------|---------------|
| `j`/`k` | カーソル移動 | カーソル移動 |
| `enter` | 展開/折りたたみ | — |
| `o` | tmux attach | — |
| `v` | 結果表示 | — |
| `e` | — | 有効/無効切替 |
| `d` | — | 削除 |
| `tab` / `1` / `2` | タブ切替 | タブ切替 |
| `q` | 終了 | 終了 |

## AI向け: CLI

CLIはAIワーカー（Claude Code）がプログラムから操作するためのインターフェース。プロンプト内で呼び出される。

### tq action done — アクション完了報告

interactive worker が作業完了時に呼ぶ。全プロンプトに記載される。

```bash
tq action done {{.Action.ID}} '{"result":"<要約>"}'
```

### tq action create — アクション作成

プロンプトやスケジュールから新しいアクションを生成する際に使う。

```bash
tq action create fix-ci --task 1 --title "CI修正" --meta '{"pr_url":"https://..."}'
```

### tq task create / update — タスク操作

classify プロンプトがタスクを生成・更新する際に使う。

```bash
tq task create "CI修正" --project hearable --url "https://..."
tq task update 3 --status done
```

### tq project create / list / delete — プロジェクト管理

```bash
tq project create hearable ~/ghq/github.com/thehearableapp/hearable-app --meta '{"gh_owner":"thehearableapp","repos":["thehearableapp/hearable-app"]}'
tq project list
tq project delete 3
```

### tq schedule — スケジュール管理

cron 式で定期実行するアクションを登録する。Queue Worker がスケジュールを監視し、時刻到達時にアクションを自動生成する。

```bash
# スケジュール登録
tq schedule create watch-gh-notifications \
  --task 181 \
  --title "Watch GitHub notifications" \
  --cron "*/5 * * * *"

# 一覧
tq schedule list

# 無効化 / 有効化
tq schedule disable <id>
tq schedule enable <id>

# 削除
tq schedule delete <id>
```

### tq dispatch — 割り込み実行

キューの順番を無視して特定アクションを即座にディスパッチする。優先実行したいアクションがある場合に使う。

```bash
tq dispatch <action_id>           # 指定アクションを即座にディスパッチ
tq dispatch                       # 次のpendingアクションをディスパッチ
tq dispatch --session worker2     # tmuxセッションを指定（デフォルト: main）
```

### その他

```bash
tq action list                   # アクション一覧（JSON）
tq action reset <id>             # failed/running → pending
tq task list                     # タスク一覧（JSON）
```

## アーキテクチャ

### 設計原則

- **SQLite が SSOT** — デイリーノートはビュー（読み取り専用）
- **Queue Worker** — 1アクション処理 → セッション終了、コンテキスト常にフレッシュ
- **AI は手足のみ** — オーケストレーションは Go、AI は `claude` ワーカー
- **TUI は読み取り専用** — 人間は TUI で状況を監視する。操作は Claude Code に自然言語で指示し、CLI 経由で実行される

### Action 状態遷移

```
                          dispatch/claim
                ┌─────────────────────────────┐
                │                             ▼
          ┌───────────┐                ┌───────────┐
          │  pending   │                │  running   │
          └───────────┘                └─────┬─────┘
                ▲                        │         │
                │                 success│         │fail
                │                        ▼         ▼
                │                  ┌────────┐  ┌────────┐
                │                  │  done   │  │ failed │
                │                  └────┬───┘  └───┬────┘
                │                       │          │
                │              on_done  │          │ reset
                │         (new action)  │          │
                └───────────────────────┘          │
                └──────────────────────────────────┘

  * running → pending: reset コマンド（tmux pane kill 付き）
  * done は terminal だが on_done で新規アクションを生成可能
```

### Worker の種類

| auto | interactive | 実行方法 |
|------|------------|---------|
| true | false | `claude -p` — stdout capture、自動 done |
| true | true | `claude --worktree --tmux` — fire-and-forget、worker が `tq action done` で報告 |
| false | * | dispatch しない → `waiting_human` |

### プロンプト

`prompts/` に frontmatter 付き markdown で定義:

```markdown
---
description: 汎用実装タスク
auto: true
interactive: true
max_retries: 0
on_done: review
---
{{.Action.Meta.instruction}}

完了したら: tq action done {{.Action.ID}} '{"result":"<要約>"}'
```

利用可能な変数: `{{.Task.ID}}`, `{{.Task.Title}}`, `{{.Task.URL}}`, `{{.Project.Name}}`, `{{.Project.WorkDir}}`, `{{.Action.ID}}`, `{{.Action.Meta.<key>}}`

## テスト

```bash
go test ./...
```
