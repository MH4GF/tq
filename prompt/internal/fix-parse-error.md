---
description: Fix a prompt parse error
mode: interactive
---
The prompt "{{index .Action.Meta "prompt_id"}}" failed to load or render for action #{{index .Action.Meta "source_action_id"}}.

Error: {{index .Action.Meta "error_message"}}

The prompt files are located in the prompts directory (~/.config/tq/prompts/).
Open the file, diagnose the parse error described above, and fix it.
Common issues include invalid YAML frontmatter, missing frontmatter delimiters (---), and Go template syntax errors.
Do not change the prompt's intended behavior — only fix the syntax/structure issue.
