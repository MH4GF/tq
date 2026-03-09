---
description: tqアクションを完了としてマークし作業結果を報告
argument-hint: "<action_id> [summary]"
allowed-tools:
  - Bash
---

# tq action done

tqアクションの完了報告を行う。セッション中の作業内容を要約し、`tq action done` コマンドで結果を記録する。

## 手順

### 1. action_id の特定

以下の優先順で action_id を特定する:

1. `$ARGUMENTS` の先頭が数値であればそれを使用する
2. 環境変数 `TQ_ACTION_ID` が設定されていればそれを使用する
3. 初回プロンプト（セッション冒頭のユーザーメッセージ）に含まれる `tq action done <数字>` パターンから抽出する
4. DB から running アクションを検索する: `tq action list --status running`

いずれでも特定できない場合、初回プロンプトから task_id と template を読み取り、アクションを作成してその ID を使用する:

```bash
tq action create <template> --task <task_id> --source human --status running
```

### 2. サマリー生成

セッション中の作業内容を振り返り、以下の内容を含む複数行のプレーンテキストサマリーを生成する。情報量は多くてよい。

- **成果**: 何を達成したか
- **プロセス**: 作業の流れ・判断・試行錯誤の詳細
- **改善提案**: 今後の改善点や気づきがあれば記述

例:
```
成果: 認証バグの修正とテスト追加

プロセス:
- ログからセッション切れが原因と特定
- トークンリフレッシュ処理を auth/refresh.go に追加
- 既存テストを拡張して再現ケースをカバー
- CI で全テスト通過を確認

改善提案:
- トークン有効期限の設定値を環境変数化すると運用しやすい
- リフレッシュ失敗時のリトライ戦略を検討したい
```

### 3. tq action done 実行

```bash
tq action done <action_id> '<summary>'
```

成功したら「action #<action_id> を完了としてマークしました。」と報告する。
