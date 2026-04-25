# tq Claude Code Plugin

tq task queue の操作用 Claude Code プラグイン。

## インストール

### marketplace source として追加

```bash
claude plugin marketplace add MH4GF/tq
```

### プラグインインストール

```bash
claude plugin install tq@tq-marketplace
```

## コマンド

### `/tq:done <action_id> [summary]`

tq アクションを完了にし、タスク全体の完了判定と follow-up アクションの提案まで行う。

tq の interactive worker 経由で起動された Claude Code セッションで使用する。

```
/tq:done           # action_id を自動検出、サマリーを自動生成
/tq:done 42        # action_id を指定、サマリーを自動生成
/tq:done 42 認証バグの修正  # action_id とサマリーを指定
```

### `/tq:failed [action_id]`

tq アクションを失敗としてマークする。完了できなかったケース（権限不足、環境破損、外部API停止、CI flake等）で使用する。失敗したアクションは `tq action reset` で pending に戻して再試行可能。

```
/tq:failed           # action_id を自動検出
/tq:failed 42        # action_id を指定
```

### `/tq:cancel [action_id]`

tq アクションをキャンセルし改善提案を記録した上で、タスク全体の完了判定と follow-up アクションの提案まで行う。

```
/tq:cancel           # action_id を自動検出
/tq:cancel 42        # action_id を指定
```

### `/tq:create-action [instruction]`

tq アクションを作成する。指示を自動推測、またはユーザーが指定可能。

### `/tq:triage [project_name]`

open タスクの棚卸し。状況確認 → 整理提案 → 実行。

## 利用する CLI コマンド

### `tq search <keyword>`

タスクタイトル、タスクメタデータ、アクションタイトル、アクション結果、アクションメタデータの横断全文検索。JSON で出力する。各結果には `project_id` が含まれる。`--jq` フラグでフィルタ可能、`--project <ID>` で特定プロジェクト内のみ検索。

```
tq search "login bug"
tq search deploy --project 1
```

## スキル

### `tq:manager`

tqタスク管理者。「タスク作って」「アクション追加して」「完了にして」「状況見せて」「割り込み実行して」「スケジュール実行したい」で発動
