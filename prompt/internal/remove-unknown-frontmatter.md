---
description: Remove unknown frontmatter fields from a prompt file
mode: noninteractive
---
The prompt file "{{index .Action.Meta "prompt_id"}}" contains unknown frontmatter fields.
Remove the following fields: {{index .Action.Meta "unknown_fields"}}

The prompt files are located in the prompts directory (~/.config/tq/prompts/).
Open the file, remove only the unknown YAML frontmatter fields listed above, and save.
Do not modify the prompt body or any known fields (description, mode, on_done, on_cancel).
