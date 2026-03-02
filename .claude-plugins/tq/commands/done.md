---
description: tqアクションを完了としてマークし作業結果を報告
argument-hint: "<action_id> [summary]"
allowed-tools:
  - Bash
---

# tq action done

tqアクションの完了報告を行う。セッション中の作業内容を要約し、`tq action done` コマンドで結果を記録する。

## 手順

### 1. TQ_DIR 環境変数の確認

`TQ_DIR` 環境変数が設定されているか確認する。未設定の場合はエラーメッセージを表示して終了する。

```bash
echo "$TQ_DIR"
```

`TQ_DIR` が空の場合、「TQ_DIR 環境変数が設定されていません。tq の interactive worker 経由で起動されたセッションでのみ使用できます。」と伝えて終了する。

### 2. action_id の特定

以下の優先順で action_id を特定する:

1. `$ARGUMENTS` の先頭が数値であればそれを使用する
2. 初回プロンプト（セッション冒頭のユーザーメッセージ）に含まれる `tq action done <数字>` パターンから抽出する
3. `TQ_DIR` 配下の DB から pending アクションを検索する: `tq --dir "$TQ_DIR" action list --status pending`

いずれでも特定できない場合、「action_id を特定できませんでした。`/tq:done <action_id>` の形式で指定してください。」と伝えて終了する。

### 3. サマリー生成

`$ARGUMENTS` に action_id 以降のテキストがあればそれをサマリーとして使用する。

なければ、セッション中の作業内容を振り返り、以下の形式で JSON サマリーを生成する:

- 形式: `{"result":"<体言止めの要約>"}`
- 体言止め（名詞で終わる）で簡潔に記述する
- 例: `{"result":"認証バグの修正とテスト追加"}`, `{"result":"API エンドポイントの新規実装"}`

### 4. tq action done 実行

```bash
tq --dir "$TQ_DIR" action done <action_id> '<json>'
```

成功したら「action #<action_id> を完了としてマークしました。」と報告する。
