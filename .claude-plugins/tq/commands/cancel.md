---
description: tqアクションをキャンセルし改善提案を記録
argument-hint: "<action_id>"
allowed-tools:
  - Bash
---

# tq action cancel

tqアクションのキャンセルを行う。なぜ不要だったかを分析し、分類ロジックへの改善提案を含むキャンセル理由を記録する。

## 手順

### 1. action_id の特定

以下の優先順で action_id を特定する:

1. `$ARGUMENTS` の先頭が数値であればそれを使用する
2. 環境変数 `TQ_ACTION_ID` が設定されていればそれを使用する
3. 初回プロンプト（セッション冒頭のユーザーメッセージ）に含まれる `tq action cancel <数字>` パターンから抽出する
4. DB から running アクションを検索する: `tq action list --status running`

いずれでも特定できない場合はユーザーに確認する。

### 2. アクションと経緯の確認

対象アクションの詳細と、同じタスクの過去アクションを確認し、このアクションが作成された経緯を理解する。

```bash
# 対象アクションの詳細を確認
tq action list --task <task_id>
```

- 同じタスクの全アクション履歴（done/failed/cancelled 含む）を確認する
- 各アクションの result を読み、どのような判断の連鎖でこのアクションが作成されたかを追跡する
- 特に直前の on_done トリガーや classify-next-action の判断が適切だったかを評価する

### 3. キャンセル理由の生成

以下の内容を含む複数行のプレーンテキストでキャンセル理由を生成する:

- **キャンセル理由**: なぜこのアクションが不要なのか
- **分類改善提案**: classify-next-action や classify-gh-notification がどう判断すればこのアクションを作成しなかったか。具体的な改善案を含める:
  - どの判断基準が不足していたか
  - どのような条件を追加すべきか
  - 類似の不要アクションを防ぐためのルール提案

例:
```
キャンセル理由: 同じタスクに対して既に review-pr アクション (#58) が running 状態で存在するのに、新たな review-pr アクション (#62) が作成された

分類改善提案:
- classify-next-action で、同一 task_id + 同一 prompt_id の pending/running アクションが既に存在する場合はスキップすべき
- HasActiveAction チェックは存在するが、classify-next-action のプロンプト内で「既存アクションの状態を確認してから判断せよ」という指示が不足している
- 条件追加案: アクション作成前に tq action list --task <task_id> で既存アクションを確認し、重複する目的のアクションがあれば作成しない
```

### 4. tq action cancel 実行

```bash
tq action cancel <action_id> '<reason>'
```

成功したら「action #<action_id> をキャンセルしました。」と報告する。
