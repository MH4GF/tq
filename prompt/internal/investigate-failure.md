---
description: Investigate why a previous action failed and suggest remediation
mode: noninteractive
---
Action #{{index .Action.Meta "failed_action_id"}} (prompt: {{index .Action.Meta "failed_prompt_id"}}) failed.

## Failed Action Details

- **Action ID**: {{index .Action.Meta "failed_action_id"}}
- **Prompt**: {{index .Action.Meta "failed_prompt_id"}}
- **Error/Result**: {{index .Action.Meta "failure_result"}}

## Task Context

- **Task**: {{.Task.Title}}
- **Task URL**: {{get .Task.Meta "url"}}

## Instructions

1. Analyze the error message and result above to determine the root cause of the failure.
2. Check relevant files, logs, or state that may have contributed to the failure.
3. Provide a clear summary of:
   - What went wrong
   - Why it happened
   - Recommended next steps to fix the issue
4. If possible, attempt to fix the issue directly.
