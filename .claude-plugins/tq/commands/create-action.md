---
description: tqアクションを作成（プロンプト自動推測 or ユーザー選択）
argument-hint: "<プロンプト名 or 実装指示>"
---

# tq action create（汎用）

ユーザーの指示やセッションのコンテキストからプロンプトを特定し、`tq action create` で worker が自動ピックアップする pending アクションを作成する。

## 手順

### 1. プロンプトの特定

まず `tq prompt list` で利用可能なプロンプト一覧を取得する。以下の優先順で決定する:

1. `$ARGUMENTS` とセッションのコンテキストから適切なプロンプトを推測する
2. 推測できない場合 → ユーザーに選択肢を提示して選んでもらう

### 2. task_id の特定

セッションの会話内容から関連するタスクを推測し、`tq task list --status open` で既存タスクを検索する。関連タスクが見つからなければ新規タスクを作成して紐付ける。

### 3. metadata 構成

プロンプトに応じて metadata を構成する:

#### instruction が必要なプロンプト: `implement`, `implement-remote`, `generic`

セッションのコンテキストと入力から、worker が実装しやすい構造化されたプロンプトを生成する。以下の項目を含める:

- **目的・ゴール**: 何を実装するか
- **関連コンテキスト**: セッション中に言及されたファイル・設計判断・技術的情報があれば記述
- **制約・注意点**: 守るべきルール・避けるべきこと

ファイル調査は行わない（worker に任せる）。セッション中の情報のみを使う。

instruction を特定できない場合はユーザーに入力を求めて終了する。

プロンプト例:
```
目的: 認証ミドルウェアの追加

関連コンテキスト:
- JWTベースの認証を採用する方針が決まっている
- auth/ ディレクトリに既存のヘルパーがある
- APIルートは cmd/server.go で定義

制約:
- 既存のテストを壊さないこと
- ミドルウェアは個別ルートに適用（グローバルではない）
```

#### その他のプロンプト: `fix-ci`, `fix-conflict`, `self-review`, `respond-review`, `merge-pr`, `alert`

metadata は `{}` で作成する（プロンプト側で `{{.Task.URL}}` 等を参照するため追加 metadata 不要）。

### 4. アクション作成

```bash
tq action create <prompt> --task <task_id> --meta '<json>' --source human --status pending
```

- `--task <task_id>` は task_id が特定できた場合のみ付与する
- `--status pending` で作成し、worker が自動ピックアップできるようにする
- `--meta` の JSON 内でプロンプト中の改行は `\n` にエスケープする

### 5. 結果報告

成功したら作成された action ID を報告する:
「`<prompt>` action #<action_id> を pending で作成しました。」

失敗したらエラー内容を報告する。
