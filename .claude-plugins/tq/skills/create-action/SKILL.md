---
description: tqアクションを作成し、別のワーカーセッションに作業を委譲する。「〜するアクション作って」「これをタスク化して」「アクション作成して」など作業の委譲を頼まれたとき、triage/gc系スキルからのフォローアップ作成時、`/tq:create-action` で発動。作業自体は実行せず、pending action を作ることがゴール。
argument-hint: "[instruction]"
allowed-tools: Bash(tq *)
---

# tq action create

**CRITICAL:**

- **This command delegates work to a separate worker session.** Creating the pending action IS the deliverable — the user already chose delegation by invoking this command.
- **DO NOT execute the instruction yourself, and DO NOT ask whether to run it now** — always just create the action.
- **DO NOT dispatch the action** (e.g. `tq action dispatch`) unless explicitly asked. Pending is the correct final state; `tq ui` picks it up.
- **After creating the action, report the ID and stop.** No follow-up execution or offers.

IMPORTANT: Run !`tq action create --help` first to understand meta format and best practices.

## Workflow

### 1. Find task_id

1. Identify the target project: check `tq project list` and match by work_dir or repo context.
2. Search within that project: `tq search "<keyword>" --project <project_id>`.
3. If a matching task exists, use its task_id. Otherwise create one with `tq task create --project <project_id>`.
4. Verify the task is still open. `tq action create` rejects parent tasks with status `done` or `archived`; if you really mean to attach a follow-up to a closed task, reopen it first with `tq task update <task_id> --status open`.

### 2. Build instruction

Consult !`tq action create --help` for CLI flag and meta format guidance.

The instruction string is the **entire** prompt the worker session sees. It launches cold: no memory of this conversation, no parent-session variables, no follow-up turn to clarify. The worker runs the configured Claude model — give it a goal and it runs its own gather → act → verify loop competently; give it a guessed step list and it follows the wrong steps faithfully and fails.

You have not read the target code in this session, and you must not — that is the worker's job. So write only what you actually know: the desired end state and why it matters. The worker derives everything else by reading the code and planning.

**Write exactly two things:**

- **Goal — the desired end state and its value.** Describe what is true once the task is done, and who benefits how. Not "fix X" / "add Y" — that is a paraphrased step, not a state. State what is true when it's right and why that matters. The concrete deliverable that ends the task (PR merged, report posted, action created) is part of the end state — name it so "done" is unambiguous.
- **Background — why now.** The trigger, the prior decision, related action / PR IDs, evidence the worker lacks. Detailed background is good: it lets the worker make judgment calls instead of asking back (it can't). Embed the evidence; don't make it reconstruct your reasoning.

**Do NOT write:** scope boundaries, step lists, target file paths, function or type signatures, "the approach". You have not read the code; anything here is a guess that becomes a literal wrong order and bloats the prompt. The worker decides all of this in its plan. The only constraints worth stating are ones the code cannot reveal — compatibility limits, "this is a plugin distributable, no user-specific paths", a sequencing requirement like "migration before code". Fold those into the Goal; don't spin up separate Scope / Constraints / Verification sections that restate each other.

**Self-contained** — no "as we discussed", "the conversation above", or parent-session variables. The worker has none of it.

**Never put `## ` markdown headers in the instruction string** — Bash safety (`Newline followed by # inside a quoted argument`) denies the `tq action create` call. Use `**Header**` (bold) for structure, as the example below does.

#### Ask first if the goal is ambiguous

If you cannot state the goal as one sentence, or it has multiple plausible readings, run `AskUserQuestion` to confirm it **before** creating the action. Skip the question when the request is already clear — don't add friction.

#### Pick effort

Pass via `--meta '{"claude_args":["--effort","<level>"]}'`. Choose by task complexity:

| Task type | Recommended `--effort` |
|---|---|
| Coding, multi-file edits, investigation, agentic work | `xhigh` (default for action work) |
| Doc tweak, single-file obvious edit | `high` |
| Trivial cleanup, just invokes another slash command | omit (defaults are fine) |
| Long-horizon deep work, hard refactor | `max` (try it, watch for overthinking) |

#### Example

Bad — work-content goal plus guessed steps the author never verified against the code:
```text
Fix the create-action skill. Open `internal/foo/bar.go`, rename `Validate`
to `ValidateV2` at line 42, update the call site in `cmd/main.go`, then run
`go test ./internal/foo/`.
```

Good — end state + value, code-reading and steps left entirely to the worker:
```text
**Goal**
`Validate` no longer exists anywhere in the repo: every caller and test uses
the renamed symbol and the suite is green. This removes the last name flagged
in the API review, so downstream teams stop guessing which `Validate` they
call. No behavior change — pure rename.

**Background**
Triggered by API review #1234; the old name collided with a stdlib helper.
```

References: [Prompting best practices](https://docs.claude.com/en/docs/build-with-claude/prompt-engineering/claude-4-best-practices) · [Building agents with the Claude Agent SDK](https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk) · [Effective context engineering for AI agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)

### 3. Create

```bash
tq action create '<instruction>' --title '<title>' --task <task_id>
```

### Worktree (optional)

Omit `--worktree` for read-only or serialized actions. Use it when the action modifies files and may run in parallel.

When set, always pair with a scope-derived name (target file, feature, bug ID, or skill) so sessions are identifiable in the TUI and `git worktree list`:

```bash
tq action create '<instruction>' --title '<title>' --task <task_id> \
  --meta '{"claude_args":["--worktree","fix-login-csrf"]}'
```
