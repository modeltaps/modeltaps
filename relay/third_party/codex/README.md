# Codex system prompt

`prompt.md` is the system prompt of the OpenAI Codex CLI, copied unchanged from
https://github.com/openai/codex/blob/2cf2a6a844f1/codex-rs/models-manager/prompt.md
(last changed 2026-06-23). It is licensed under the Apache License, Version 2.0; the license
and the project's NOTICE are in this directory.

The gateway sends it as `base_instructions` for every model listed by `/codex/v1/models`
(`relay/agent_models.go`): Codex replaces its own prompt with the catalog's, so the catalog
carries the prompt Codex itself would use.

To update it, copy the file again from the Codex repository and change the commit above.
