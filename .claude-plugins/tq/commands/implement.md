---
description: ユーザーの実装指示を整理しworker用のimplementアクションを作成
argument-hint: "<実装指示>"
allowed-tools:
  - Bash
---

# tq action create (implement)

ユーザーの実装指示を構造化されたプロンプトとして整理し、`tq action create --template implement` で worker が自動ピックアップする pending アクションを作成する。

## 手順

### 1. 入力の特定

以下の優先順で実装指示を特定する:

1. `$ARGUMENTS` が空でなければそれを実装指示として使用する
2. `$ARGUMENTS` が空の場合、セッション中の会話内容（ユーザーの発言、議論の文脈）から実装指示を推測して構成する

いずれでも指示を特定できない場合は「実装指示を特定できませんでした。`/tq:implement <実装指示>` の形式で指定してください。」と伝えて終了する。

### 2. task_id の特定

セッションの会話内容から関連するタスクを推測し、`tq --dir "$TQ_DIR" task list --status open` で既存タスクを検索する。関連タスクが見つからなければ新規タスクを作成して紐付ける。

### 3. プロンプト構成

セッションのコンテキストと入力から、worker が実装しやすい構造化されたプロンプトを生成する。以下の項目を含める:

- **目的・ゴール**: 何を実装するか
- **関連コンテキスト**: セッション中に言及されたファイル・設計判断・技術的情報があれば記述
- **制約・注意点**: 守るべきルール・避けるべきこと

ファイル調査は行わない（worker に任せる）。セッション中の情報のみを使う。

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

### 4. アクション作成

```bash
tq --dir "$TQ_DIR" action create implement --task <task_id> --meta '{"instruction":"<構造化されたプロンプト>"}' --source human --status pending
```

- `--task <task_id>` は task_id が特定できた場合のみ付与する
- `--status pending` で作成し、worker が自動ピックアップできるようにする
- `--meta` の JSON 内でプロンプト中の改行は `\n` にエスケープする

### 5. 結果報告

成功したら作成された action ID を報告する:
「implement action #<action_id> を pending で作成しました。」

失敗したらエラー内容を報告する。
