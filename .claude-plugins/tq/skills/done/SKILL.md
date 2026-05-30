---
description: Mark a tq action as done, then judge task-level completion and propose follow-up actions when work remains
argument-hint: "<action_id> [summary]"
allowed-tools: Bash(tq *), Bash(gh pr view *), Bash(git status *), Bash(git log *)
---

# tq action done

Before you run `tq action done`, settle every `remaining` line — see
**Settle `remaining` before done** below. A `done` whose `remaining` still
holds bare prose is the failure this skill exists to prevent: that work
becomes invisible to `tq action list` and `/tq:triage`, so nobody picks it up.

## Find action_id

1. `$ARGUMENTS` if numeric
2. The action/task IDs stated under the `## tq action context` heading appended to the prompt (e.g. "You are executing **action #123** (task #45).")
3. Search non-terminal actions: `tq action list --status running` or `tq action list --status dispatched`
4. If none works, ask the user

## Settle `remaining` before done

Draft the `remaining` section first, then triage each line. A `remaining`
entry is a promise that something still needs doing — a promise nobody can
keep if it lives only inside a closed action's result text. So every line
must leave this step in one of two shapes:

1. **Info-only — no future work.** Context, a known limitation, a design
   note someone might want later. There is nothing to *do*, so it does not
   belong in `remaining`. Move it into `decisions` or `outcome` and drop the
   line.
2. **Work still owed.** A concrete next step (post-merge `terraform apply`,
   an unaddressed review thread, a follow-up refactor). File it as a tracked
   action now and link it:

   ```bash
   tq action create '<self-contained instruction>' --title '<title>' --task <task_id>
   ```

   Then append the returned id to the line: `- <what remains> → #<id>`.

Get `<task_id>` from the `## tq action context` heading, or
`tq action get <action_id> --jq .task_id`. The parent task is still open
(work remains), so `tq action create` accepts it.

Write the follow-up instruction the way `/tq:create-action` would: lead with
the goal and the done condition, say why it matters, name the concrete
deliverable and verification, and keep it self-contained — the worker starts
cold with no memory of this session. You only have `Bash(tq *)` here, so call
`tq action create` directly rather than the `/tq:create-action` skill.

**If `tq action create` fails** (e.g. parent task unexpectedly closed, DB
error): do not run `tq action done`. The whole point is that no work goes
untracked, and a `done` with an unfiled remaining entry breaks that. Record
the create failure as the blocker via `/tq:failed` instead.

After this step, `remaining` is either empty (all done) or every line ends in
`→ #<id>`. No bare prose survives.

## Merge guard — a code-change action needs a merged PR

Run this before `tq action done`. It is a mechanical gate, not a reminder:
an action that produced code changes is **not done until its PR is merged**
(auto-memory `feedback_task_done_after_merge`). #4666 marked done with a
permission-denied `gh pr create` and no PR — the fix never landed and was
re-discovered three times. This gate prevents that.

**1. Decide whether this is a code-change action.** Treat it as code-change
if **any** of these hold (when in doubt, treat it as code-change — a spurious
prompt on a borderline doc PR is cheaper than a silently-lost fix):

- the drafted result mentions a PR, pull request, branch, merge, or commit
- the result names an edited source/config file (`.go`, `go.mod`,
  `*.yml`, scripts, plugin/command/skill markdown, etc.)
- `git status --porcelain` shows tracked modifications, or
  `git log origin/main..HEAD --oneline` lists commits not on `main`

Not a code-change action: reports, investigations, triage, or deliverables
whose only output is created/updated tq actions. These have no PR by
design — skip the rest of this gate and go to **Execute**.

**2. Require a merged PR.** For a code-change action, the `artifacts:`
section MUST carry a PR reference (`#<n>` or a PR URL). Verify it is merged:

```bash
gh pr view <n> --json state,mergedAt
```

Proceed to **Execute** only when `state` is `MERGED` and `mergedAt` is
non-null.

**3. If there is no merged PR, do NOT run `tq action done`.** Surface the
gap explicitly and route to recovery instead:

- PR open / CI pending / not yet merged → finish the merge (resolve CI,
  merge per repo policy), then re-run `/tq:done`. The task is not done
  until the PR is merged.
- A real blocker prevented the PR — e.g. `gh pr create` was
  permission-denied, the environment is broken, CI is irrecoverably red.
  This is a **blocker, not a done**: run `/tq:failed` (retry possible) or
  `/tq:cancel` (work no longer needed). A permission-denied `gh pr create`
  is explicitly `/tq:failed`, never `/tq:done`.

## Execute

IMPORTANT: Run !`tq action done --help` for the full result format guidance.

`tq action done <action_id> '<result>'`

Result uses structured sections: outcome, decisions, artifacts, remaining.
Describe what changed, not the process steps — session logs already capture that.

Example of a compliant result with work still owed:

```
outcome: Added Terraform module for the new SQS queue; PR #142 open
decisions: Chose SQS over SNS — consumers need at-least-once with replay
artifacts: PR #142, infra/sqs/main.tf
remaining:
  - terraform apply after PR #142 merges → #4930
```

If nothing remains, omit the `remaining` section entirely.

## After marking done: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + re-read the `remaining` you just wrote.
2. Classify the task:
   - **Done** — no remaining work, no external dependency.
   - **Follow-up needed** — speculative task-level next steps not already
     captured as a `remaining → #<id>` line (e.g. an improvement idea worth
     a separate task, a broader refactor). Propose 1–2 candidates (title +
     one-line purpose) and ask the user to create via `/tq:create-action`.
     Do not auto-create *these* — they are judgment calls, unlike the
     remaining-entry tracking actions above, which you file yourself because
     they record work you already know is owed.
   - **External blocker only** — the residue is a PR merge, review reply,
     upstream release, or another task. State explicitly: "Task #<id> stays
     open, waiting on <dep>." If that residue carries future work of its own
     (e.g. a post-merge apply), it is a `remaining` line and must already
     have its `→ #<id>` from the step above.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the result's `remaining` section has incomplete signals, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running/dispatched) action with the same purpose already exists for this task.
