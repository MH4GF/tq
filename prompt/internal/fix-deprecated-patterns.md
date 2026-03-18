---
description: Fix deprecated template patterns in a prompt file
mode: interactive
---
The prompt file "{{index .Action.Meta "prompt_id"}}" uses deprecated template patterns.
Replace the following patterns: {{index .Action.Meta "deprecated_patterns"}}

Migration guide:
- Replace `{{"{{"}} .Task.URL {{"}}"}}` with `{{"{{"}} index .Task.Meta "url" {{"}}"}}`

The prompt files are located in the prompts directory (~/.config/tq/prompts/).
Open the file, replace the deprecated patterns as described above, and save.
Do not modify the prompt body or any known fields (description, mode, on_done, on_cancel) beyond the pattern replacements.
