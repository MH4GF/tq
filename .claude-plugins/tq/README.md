# tq Claude Code Plugin

Claude Code plugin for operating the tq task queue.

## Installation

### Add as a marketplace source

```bash
claude plugin marketplace add MH4GF/tq
```

### Install the plugin

```bash
claude plugin install tq@tq-marketplace
```

## Skills

All tq operations are packaged as Agent Skills. Each one triggers from natural
language **and** from the matching slash invocation `/tq:<name>` — they are
equivalent. The slash form is still the explicit entry point used by the
dispatch worker prompt and by other skills; natural language lets the same flow
fire without remembering a command name. Each description below is reproduced
verbatim from the skill's `SKILL.md` frontmatter.

### `tq:create-action` — `/tq:create-action [instruction]`

tqアクションを作成し、別のワーカーセッションに作業を委譲する。「〜するアクション作って」「これをタスク化して」「アクション作成して」など作業の委譲を頼まれたとき、triage/gc系スキルからのフォローアップ作成時、`/tq:create-action` で発動。作業自体は実行せず、pending action を作ることがゴール。

`skills/create-action/SKILL.md`

### `tq:done` — `/tq:done <action_id> [summary]`

完了したtqアクションの結果を記録し、タスクの完了判定とフォローアップ作成を行う。「アクション完了して」「doneにして」「結果を記録して」、ワーカーセッションが作業を終えたとき、`/tq:done` で発動。code変更を伴うアクションはPRがマージされて初めて done にできる。

`skills/done/SKILL.md`

```
/tq:done           # auto-detect action_id, auto-generate summary
/tq:done 42        # specify action_id, auto-generate summary
/tq:done 42 Fix auth bug  # specify action_id and summary
```

### `tq:failed` — `/tq:failed [action_id]`

tqアクションを失敗として記録し、リトライや別アプローチのフォローアップを判断する。権限不足・環境破損・外部API障害・CI flake などで完了できなかったとき、「失敗として記録して」「failedにして」、`/tq:failed` で発動。failed アクションは `tq action reset` で再試行できる。

`skills/failed/SKILL.md`

```
/tq:failed           # auto-detect action_id
/tq:failed 42        # specify action_id
```

### `tq:cancel` — `/tq:cancel [action_id]`

tqアクションを改善提案つきでキャンセルし、タスクの完了判定とフォローアップを行う。「このアクションはもう不要」「キャンセルして」「cancelして」など作業が不要・重複・陳腐化したとき、`/tq:cancel` で発動。

`skills/cancel/SKILL.md`

```
/tq:cancel           # auto-detect action_id
/tq:cancel 42        # specify action_id
```

### `tq:triage` — `/tq:triage [project_name]`

オープンなtqタスクを棚卸し・整理する。各タスクの状態をレビューし、クリーンアップを提案・実行する。「タスク整理して」「オープンタスク見直して」「triageして」、`/tq:triage` で発動。

`skills/triage/SKILL.md`

### `tq:manager`

tqタスク管理者。タスク/アクションの一覧・状況確認・ディスパッチ・スケジュール運用のハブ。「状況見せて」「タスク一覧」「割り込み実行して」「スケジュール実行したい」「タスク作って」で発動。アクションの委譲作成は tq:create-action、完了/失敗/キャンセルの記録は tq:done / tq:failed / tq:cancel、タスク棚卸しは tq:triage が担当する。

`skills/manager/SKILL.md`

When creating actions/schedules, prefer the `experimental_bg` or `interactive` dispatch mode and avoid `noninteractive` (it draws from the capped Agent SDK credit / API billing). See [Best practices — Dispatch mode selection](../../docs/best-practices.md#dispatch-mode-selection) for the full decision rule.

### `tq:investigate-incidents`

tqキューに溜まった失敗actionとpermission denialを横断的に診断する。「インシデント調べて」「最近の失敗まとめて」「permission blockの傾向見せて」「/tq:investigate-incidents」で発動

`skills/investigate-incidents/SKILL.md`

Replaces the per-event auto-generated follow-up actions that previously fired on every failure and every permission denial. Recommended usage is a daily schedule:

```bash
tq schedule create --instruction '/tq:investigate-incidents' --task <task_id> --cron '0 9 * * *' \
    --title 'Daily incident review'
```

Run `tq schedule create --help` for available flags.

## CLI commands used

### `tq search <keyword>`

Full-text search across task titles, task metadata, task status change reasons, action titles, action results, and action metadata. Output is JSON. Each result includes `project_id`. Filter with `--jq`, or scope to a single project with `--project <ID>`.

```
tq search "login bug"
tq search deploy --project 1
```

## hooks

### `SessionStart`

Runs `tq internal claude-session-record` to record the `session_id` issued by Claude Code into the action metadata's `claude_session_id`.

- The startup env variable `TQ_ACTION_ID` identifies the target action 1:1. Only Claude sessions launched via tq dispatch are recorded.
- No side effects on manual claude launches without `TQ_ACTION_ID` (silent exit).
- When `CLAUDE_CODE_REMOTE=true` (Claude Code on the web / Cloud Routines) is set, `executor=cloud` is also recorded into metadata. The reaper uses this value to unconditionally skip cloud-executed actions (local session log liveness checks do not apply to them).
- Hook failures never disrupt the Claude session (always exits 0).
