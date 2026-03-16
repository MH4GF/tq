---
description: Investigate why a previous action failed and suggest remediation
mode: noninteractive
---
Action #{{index .Action.Meta "triggered_by_action_id"}} failed with status "failed".

## Failed Action Details

- **Action ID**: {{index .Action.Meta "triggered_by_action_id"}}
- **Error/Result**: {{index .Action.Meta "predecessor_result"}}

## Task Context

- **Task**: {{.Task.Title}}
- **Task URL**: {{.Task.URL}}

## Instructions

1. Analyze the error message and result above to determine the root cause of the failure.
2. Check relevant files, logs, or state that may have contributed to the failure.
3. Provide a clear summary of:
   - What went wrong
   - Why it happened
   - Recommended next steps to fix the issue
4. If possible, attempt to fix the issue directly.
