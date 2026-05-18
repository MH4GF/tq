---
description: tqキューに溜まった失敗actionとpermission denialを横断的に診断する。「インシデント調べて」「最近の失敗まとめて」「permission blockの傾向見せて」「/tq:investigate-incidents」で発動
context: fork
allowed-tools: Bash(tq *), Read, Grep, Glob
---

# tq Investigate Incidents

You review recent failed actions and permission denials across the whole queue. The goal is **cross-event judgment**, not per-event firefighting.

This skill replaces a retired per-event path that auto-generated one follow-up action per failure or per denial. That path ran roughly 150 Claude sessions per month and accumulated remediations (SKILL.md edits, settings tweaks) without ever verifying whether the remediations actually fixed the root cause. The point of doing this in batch is precisely to look across events for patterns that per-event analysis was structurally blind to.

## What "an incident" means here

Two shapes:

1. **Failed action** — `tq action list --status failed` returns these. The `result` field carries the failure message.
2. **Permission-blocked action** — an action that ran to `done` but had tool-use denials along the way. Denial summaries are stamped onto the source action's metadata under the `permission_denials` key (introduced 2026-05-14, replacing the old auto-generated `is_permission_block` follow-up).

Legacy follow-ups (`is_investigate_failure: true`, `is_permission_block: true`) still exist in the DB from before the retirement. Treat them as **history**: they tell you what was already remediated for past events.

## How to think about this

The reason per-event analysis was retired is that it kept producing locally plausible fixes — adding a `Bash(...)` allowlist entry, tightening a SKILL.md instruction, writing a defensive memory — without ever validating whether the same pattern recurred afterward. So defensive restrictions piled up while the underlying problem (often a harness race, a model/effort mismatch, or a structural permission-matcher quirk) stayed.

Use this skill to do what the per-event path couldn't:

- **Cluster.** Group incidents by signal, not by ID. Failure messages, denied tool patterns, source action context, time-of-day, schedule/worker identity — whichever axis is informative. Some clusters span both failures and denials with a shared root cause.
- **Cross-check prior remediations.** For each cluster, search history: was this same pattern remediated before? If yes, and it's recurring, the prior remediation was wrong — the lesson there is more valuable than another defensive block. Surface it.
- **Distinguish systemic from one-off.** A single odd failure usually doesn't need action. A cluster of 10 over a week usually does, and that action should be at the level of the cluster (one fix, one well-reasoned change) — not 10 separate per-event patches.
- **Be skeptical of "just add an allow rule" / "just add a禁止 to SKILL.md".** Sometimes that's the right answer. Often it's not. Ask whether the rule will still be obviously correct in three months, or whether it's papering over a structural issue.

## How to start

Run the CLI to see what's available, then read what matters:

```bash
tq action list --status failed --jq '.[] | {id, task_id, created_at, title, result_head: (.result // "")[0:200]}'
tq action list --jq '.[] | select((.metadata | fromjson? | .permission_denials) != null) | {id, task_id, created_at, title, denials: (.metadata | fromjson? | .permission_denials)}'
```

Default time window: roughly the last 24 hours when run on a schedule, or whatever window the caller asks for. Don't hold yourself to a strict window — if a cluster clearly extends further back, follow it.

For cross-referencing prior remediations:

```bash
tq action list --jq '.[] | select(.metadata | contains("is_investigate_failure") or contains("is_permission_block")) | {id, created_at, title, result_head: (.result // "")[0:300]}'
```

Use `tq search` for free-text searches across older history when you suspect a remediation but don't see it in the recent list.

## Output

Produce a markdown report. Suggested skeleton — adapt freely:

```
# Incident review (window: <range>)

## Clusters
### <cluster name> (<count>, e.g. 14×)
- pattern: <what ties them together>
- representative IDs: #N, #M, #...
- prior remediation: <link or "none found">
- recurrence-after-remediation: <yes/no/uncertain>
- assessment: <what this likely is — harness race, model mismatch, structural, ...>
- recommendation: <do nothing / file fix action / flag a prior remediation as ineffective>

## One-offs
- #N: <one-line>
- #M: <one-line>

## Followups created
- action #X: <title> (only if you actually created one)
```

When recommendation = "file fix action", use `tq action create` to spawn it. Make the new action's instruction concrete and bounded — that action will run Claude separately, so don't bundle multiple unrelated fixes into one. **Default to no follow-up** unless the recommendation is high-confidence; over-creation re-introduces the cost problem this skill was built to solve.

## Filtering tip

Use the built-in `--jq` flag on `tq` commands. Don't pipe to external `jq` / `python` — piped commands trigger permission prompts that break scheduled runs.

```bash
# Good
tq action list --status failed --jq '.[] | select(.created_at > "2026-05-13")'

# Bad — triggers approval prompt
tq action list --status failed | jq '.[] | select(.created_at > "2026-05-13")'
```
