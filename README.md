# tq — Task Queue

タスクとアクションをSQLiteで管理し、Claude Codeワーカーで自動処理するタスクキュー。

## セットアップ

```bash
cd tq && go install .
```

`TQ_DIR` の解決: `--dir` フラグ > `TQ_DIR` 環境変数 > git root + `/tq/`

### プロジェクト登録

初回セットアップ時にプロジェクトを登録する:

```bash
tq project create --name myapp --work-dir ~/ghq/github.com/org/myapp --metadata '{"gh_owner":"org","repos":["org/myapp"]}'
```

登録済みプロジェクトの確認:

```bash
tq project list
```

## 人間向け: TUI (`tq ui`)

人間はTUIで操作する。`tq ui` が唯一のエントリポイント。

```bash
tq ui
```

バックグラウンドで以下が自動実行される:
- **Ralph Loop**: pending アクションを検出して自動ディスパッチ
- **Watch**: GitHub通知を取得して task/action を自動生成

### タスクを作る

Tasks タブ (`2`) → `c` → プロジェクト選択 → タイトル入力 → URL入力(任意)

### アクションを追加する

Tasks タブでタスクにカーソル → `n` → テンプレート選択 → (implement の場合) instruction 入力

追加されたアクションは Ralph Loop が自動で拾って実行する。

### 実行状況を確認する

- **Queue タブ** (`1`): アクション一覧。ステータス・結果を確認
- **Tasks タブ** (`2`): Project → Task → Actions のツリービュー

### 人間の判断が必要な時

worker が失敗すると `waiting_human` になる。Queue タブで:
- 該当アクションにカーソルを合わせると失敗理由が表示される
- `a`: approve（pending に戻してリトライ）
- `x`: reject（failed にして終了）

### キーバインド

| キー | Queue タブ | Tasks タブ |
|------|-----------|-----------|
| `j`/`k` | カーソル移動 | カーソル移動 |
| `enter` | — | 展開/折りたたみ |
| `a` | approve | — |
| `x` | reject | — |
| `s` | — | ステータス変更 |
| `c` | — | タスク作成 |
| `n` | — | アクション追加 |
| `r` | 再読込 | 再読込 |
| `tab` / `1` / `2` | タブ切替 | タブ切替 |
| `q` | 終了 | 終了 |

### デイリーノートに反映する

```bash
tq view                  # markdown プレビュー
tq view --inject         # デイリーノートに書き込み
```

## AI向け: CLI

CLIはAIワーカー（Claude Code）がプログラムから操作するためのインターフェース。テンプレート内で呼び出される。

### tq action done — アクション完了報告

interactive worker が作業完了時に呼ぶ。全テンプレートに記載される。

```bash
tq --dir {{.TQDir}} action done {{.Action.ID}} '{"result":"<要約>"}'
```

### tq action create — アクション作成

classify テンプレートが通知からアクションを生成する際に使う。

```bash
tq action create --template fix-ci --task 1 --meta '{"pr_url":"https://..."}'
```

### tq task create / update — タスク操作

classify テンプレートがタスクを生成・更新する際に使う。

```bash
tq task create --project hearable --title "CI修正" --url "https://..."
tq task update --id 3 --status done
```

### tq project create / list / delete — プロジェクト管理

```bash
tq project create --name hearable --work-dir ~/ghq/github.com/thehearableapp/hearable-app --metadata '{"gh_owner":"thehearableapp","repos":["thehearableapp/hearable-app"]}'
tq project list
tq project delete --id 3
```

### その他

```bash
tq action list                   # アクション一覧
tq action approve <id>           # waiting_human → pending
tq action reject <id>            # waiting_human → failed
tq task list                     # タスク一覧
tq project list                  # プロジェクト一覧
tq project create ...            # プロジェクト作成
tq project delete --id <id>      # プロジェクト削除
tq status                        # キュー集計
tq dispatch                      # 1アクション処理
tq run                           # Ralph Loop（継続 dispatch）
tq watch                         # GitHub通知取得 → classify
```

## アーキテクチャ

### 設計原則

- **SQLite が SSOT** — デイリーノートはビュー（読み取り専用）
- **Ralph Loop** — 1アクション処理 → セッション終了、コンテキスト常にフレッシュ
- **AI は手足のみ** — オーケストレーションは Go、AI は `claude` ワーカー

### Action 状態遷移

```
                    ┌────────────────────────┐
                    │        retry           │
                    │   (retries remain)     │
                    ▼                        │
 ┌─────────┐  dispatch  ┌─────────┐    fail │
 │ pending  │───────────▶│ running │─────────┘
 └─────────┘            └────┬────┘
      ▲                  │        │
      │            success│        │fail (no retries)
      │                  │        │
      │             ┌────▼──┐  ┌──▼───────────┐
      │             │ done  │  │waiting_human  │
      │             └───────┘  └──┬────────┬──┘
      │                           │        │
      │          human approve    │        │ reject
      └───────────────────────────┘        │
                                    ┌──────▼───┐
                                    │  failed   │
                                    └──────────┘
```

### Worker の種類

| auto | interactive | 実行方法 |
|------|------------|---------|
| true | false | `claude -p` — stdout capture、自動 done |
| true | true | `claude --worktree --tmux` — fire-and-forget、worker が `tq action done` で報告 |
| false | * | dispatch しない → `waiting_human` |

### テンプレート

`templates/` に frontmatter 付き markdown で定義:

```markdown
---
description: 汎用実装タスク
auto: true
interactive: true
max_retries: 0
---
{{.Action.Meta.instruction}}

完了したら: tq --dir {{.TQDir}} action done {{.Action.ID}} '{"result":"<要約>"}'
```

利用可能な変数: `{{.Task.ID}}`, `{{.Task.Title}}`, `{{.Task.URL}}`, `{{.Project.Name}}`, `{{.Project.WorkDir}}`, `{{.Action.ID}}`, `{{.Action.Meta.<key>}}`, `{{.TQDir}}`

## テスト

```bash
go test ./...
```
