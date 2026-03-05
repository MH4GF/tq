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

### `/tq:done [action_id] [summary]`

tq アクションを完了としてマークし、作業結果を報告する。

tq の interactive worker 経由で起動された Claude Code セッションで使用する。

```
/tq:done           # action_id を自動検出、サマリーを自動生成
/tq:done 42        # action_id を指定、サマリーを自動生成
/tq:done 42 認証バグの修正  # action_id とサマリーを指定
```

### `/tq:implement`

ユーザーの実装指示を整理し worker 用の implement アクションを作成する。

## スキル

### `tq:manage`

tq CLI によるタスク・アクション管理スキル。「タスク作って」「アクション追加して」「完了にして」「状況見せて」で発動する。

リファレンス:
- タスク管理: `skills/manage/tasks.md`
- アクション管理: `skills/manage/actions.md`
- ビュー・状況確認: `skills/manage/view.md`
