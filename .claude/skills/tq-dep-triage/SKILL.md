---
name: tq-dep-triage
description: Rescue tq actions stuck pending forever because a completion dependency ended failed or cancelled. A blocked action is only released when every blocker reaches a successful terminal state, so a failed/cancelled blocker strands the dependent indefinitely. Use this whenever asked to triage blocked/stuck/stalled pending actions, find actions waiting on a dead dependency, audit blocked_by chains, or unblock the dispatch queue — including the scheduled "/tq-dep-triage" run.
context: fork
allowed-tools: Bash(tq action list*), Bash(tq action get*), Bash(tq task get*)
---

# tq dependency triage

Completion dependencies gate dispatch: an action stays `pending` until **every**
blocker reaches a successful terminal state (`action`=`done`,
`task`=`done`/`archived`). By design there is no automatic escape — a blocker
that ends `failed` or `cancelled` leaves the dependent stranded forever. This
skill finds those dead ends and hands the user concrete, ready-to-run rescue
commands. It does **not** mutate anything itself; the human decides.

## What to do

Pull the pending actions and read their `blocked_by`:

```bash
tq action list --status pending
```

Each `blocked_by` entry has `type`, `id`, `satisfied`, `blocker_status`. The
ones that matter are entries where the blocker has reached a non-success
terminal state (`blocker_status` = `failed` or `cancelled`) — those will never
flip to satisfied on their own, so the dependent is dead in the water. An
unsatisfied blocker that is still `pending`/`running`/`dispatched`, or a task
blocker still `open`, is just normal waiting — leave it alone.

For each stranded action, figure out the most useful recovery. Use judgment
rather than a fixed recipe — the right move depends on whether the failed work
still needs doing and whether it was already retried elsewhere:

- A failed blocker action may have been **resumed**, which creates a *new*
  action id; the original `dep_id` will never go `done`. Look for a resumed
  descendant by following `metadata.parent_action_id` from later actions back
  to the failed blocker (`tq action get <id>` / scan `tq action list`). If a
  live or done descendant exists, re-pointing the dependency at it is usually
  the cleanest fix.
- If the prerequisite work genuinely still needs to happen, reviving the
  original blocker (so it reaches `done` under the same id) keeps the
  dependency intact with no edits to the dependent.
- If the dependency is stale — the dependent no longer actually needs that
  prerequisite — dropping it is fine.
- If the dependent itself is now moot, retiring it clears the queue.

Surface the failed blocker's own result/reason (`tq action get <id>`) so the
user understands *why* it died before choosing.

## Output

Report only the genuinely stranded actions (skip normal waiting). For each,
give: the blocked action (id + title), the failed/cancelled blocker (id +
status + why it died), any resumed descendant you found, and the rescue
options as copy-paste commands the user can run. Recommend the one you think
fits, but present the alternatives and let the user choose — do not run them.

Rescue commands (fill in real ids):

```
# re-point to a resumed/replacement action
tq action update <blocked> --clear-deps --blocked-by-action <newid>

# revive the original blocker so it completes under the same id
tq action reset <failed_blocker>

# drop a dependency that is no longer needed
tq action update <blocked> --clear-deps

# retire a dependent that is now moot
tq action cancel <blocked>
```

If nothing is stranded, say so plainly — a clean queue is the expected steady
state, not a finding to manufacture.
