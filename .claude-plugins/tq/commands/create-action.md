---
description: Create a tq action (auto-infer instruction or let user specify)
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
Use only information from the current session — do not investigate files (that is the worker's job).

The instruction string is the **entire** prompt the worker session sees. It launches cold: no memory of this conversation, no parent-session variables, no follow-up turn to clarify. Write it like a brief to a new teammate.

The worker is Claude Opus 4.7. Two behaviors to design around:

- **Literal instruction following** — wrong paths, wrong function names, and wrong steps get executed wrongly. Under-specified scope causes drift; over-specified steps cause faithful failure.
- **Goal-directed autonomy** — given a goal + constraints + verification, 4.7 runs its own gather → act → verify loop competently. Trust it to find the right files.

#### Pre-flight: ask first if the goal is ambiguous

If you cannot write the goal as one sentence — or scope / done condition has multiple plausible readings — run `AskUserQuestion` to confirm goal, scope, and done condition **before** creating the action. Skip the question when the request is already clear; don't add friction.

#### Checklist — confirm each before calling `tq action create`

1. **Goal first, steps last** — lead with goal + done condition. Only enumerate steps when ordering is genuinely required (e.g. migration must precede code change). 4.7 will invent the right steps from a clear goal; it cannot recover from a wrong step list.
2. **Why it matters / background** — the trigger, prior decision, related action / PR ID. Lets the worker make judgment calls instead of asking back (which it can't).
3. **Concrete deliverable** — what artifact ends the task (PR open, file edited, report posted, action created). "Done" must be unambiguous.
4. **Scope and constraints** — what's in / out, what NOT to touch, compatibility limits. 4.7 respects scope literally.
5. **Pointers as hints, not commands** — file paths, function names, PR / issue numbers belong as "starting point — verify against current code", not as `path:line` editing orders. Add "if the hint is stale, prefer the actual code" so the worker overrides bad pointers.
6. **Verification step** — how the worker confirms success (`go test ./...`, CI green, hit a URL, `tq action list`). Without this the worker stops at "looks good".
7. **Positive framing** — say what to do. "Commit per logical change" beats "don't make giant commits".
8. **Output format if it matters** — PR description shape, comment template, report sections. Skip when the deliverable already implies format.
9. **Self-contained** — no references to "the conversation above", "as we discussed", or parent-session variables. The worker has none of it.

**Never put `## ` markdown headers inside the instruction string.** Bash's built-in safety (`Newline followed by # inside a quoted argument`) denies the `tq action create` call — use `**Header**` (bold) for structure, as the examples below do.

#### Pick effort

Pass via `--meta '{"claude_args":["--effort","<level>"]}'`. Choose by task complexity:

| Task type | Recommended `--effort` |
|---|---|
| Coding, multi-file edits, investigation, agentic work | `xhigh` (default for action work) |
| Doc tweak, single-file obvious edit | `high` |
| Trivial cleanup, just invokes another slash command | omit (defaults are fine) |
| Long-horizon deep work, hard refactor | `max` (try it, watch for overthinking) |

#### Examples

**Pair 1 — vague vs goal-clear**

Bad:
```text
Fix the create-action skill.
```

Good:
```text
**Goal**
Improve the "Build instruction" section of `.claude-plugins/tq/commands/create-action.md`
so generated worker instructions consistently include goal, context, and verification.

**Why**
Worker sessions currently get terse instructions and waste turns inventing objectives.

**Constraints**
- Don't change the `CRITICAL` block — its delegation intent is load-bearing.
- Extend the existing section; don't add new top-level sections.

**Done**
PR open against `main`, CI green, `/quality-review` recorded.
```

**Pair 2 — over-prescribed steps vs goal-oriented**

Bad:
```text
1. Open `internal/foo/bar.go` and rename `Validate` to `ValidateV2` at line 42.
2. Open `cmd/main.go` line 88 and update the call site.
3. Open `internal/foo/bar_test.go` line 15 and rename the test.
4. Run `go test ./internal/foo/`.
```

Good:
```text
**Goal**
Rename `Validate` → `ValidateV2` across the repo so all callers and tests follow.

**Hints (verify against current code; prefer real code if stale)**
- Definition is somewhere under `internal/foo/`.
- Likely callers in `cmd/` and tests in `internal/foo/*_test.go`.

**Done**
- Every reference renamed; no `Validate` symbol survives `grep -r '\bValidate\b'`.
- `go test ./...` green.
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
