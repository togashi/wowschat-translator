# AGENTS.md

This file captures repository-specific guidance for GPT-style coding agents.

## Project Summary

- Project: wowschat-translator (Go)
- Purpose: Translate World of Warships in-game chat.
- Default engine: DeepL
- Optional advanced engines: GPT (OpenAI Responses API), Claude (Anthropic Messages API)

## Core Architecture

- Entry point: cmd/wowschat-translator/main.go
- Config loader: internal/config/config.go
- Server: internal/server/server.go
- Translator interface + shared formatting: internal/translator/translator.go
- DeepL implementation: internal/translator/deepl_translator.go
- GPT implementation: internal/translator/gpt_translator.go
- Claude implementation: internal/translator/claude_translator.go
- Shared prompt loader: internal/translator/prompt.go
- Trace model: internal/translator/trace.go

## Configuration Rules

- Priority: CLI flags > environment variables > config file > defaults
- Primary DeepL key: deepl_api_key
- Legacy DeepL key fallback: api_key
- GPT config keys: openai_api_key, openai_model, openai_prompt_file, openai_temperature
- Claude config keys: anthropic_api_key, anthropic_model, anthropic_prompt_file, anthropic_temperature
- Supported engines: deepl, gpt, claude

## Prompt Behavior (GPT / Claude)

- Embedded default prompt file: internal/translator/prompts/gpt_system_prompt.txt (shared by GPT and Claude)
- External prompt override can be provided with openai_prompt_file (GPT) or anthropic_prompt_file (Claude)
- External prompt supports placeholders:
  - {{PASSTHROUGH}}
  - {{GLOSSARY}}
- If external prompt is active and placeholders are absent, passthrough/glossary are not auto-appended.
- If embedded default prompt is used, passthrough/glossary are auto-appended for compatibility.

## Passthrough/Glossary/Expand Notes

- Passthrough rule syntax:
  - `word` (plain) — word-boundary match, case-insensitive (`\b` regex)
  - `RPF: *` (trailing `*`) — prefix match against the full input string start
  - `/pattern/` or `/pattern/flags` — regex match (Go regexp syntax)
- Plain words use case-insensitive word-boundary matching by default.
- Passthrough rules are cached in GPT and Claude translators.
- Expand (`expand` config) rewrites abbreviations to full words before translation using `\b` word-boundary matching (case-insensitive). Applied before passthrough masking.
- Processing order: expand → passthrough masking → LLM translation → restore masked segments.
- Glossary entries are rendered in sorted key order for deterministic prompts.
- If passthrough masking covers the entire input, LLM API call is skipped.

## First-Run UX and Packaging

- App can auto-create config.yaml when missing.
- Manual config generation is available via: --init-config
- Release workflow publishes a zip package including exe + config.yaml.example.

## Documentation Conventions

- Keep README.md and README.ja.md aligned for behavior and setup steps.
- Position GPT as optional advanced functionality.
- Keep DeepL as default/recommended path.

## Validation Checklist

Before finalizing changes:

1. Run: go test ./...
2. If config behavior changed, ensure README.md + README.ja.md + config.yaml.example remain consistent.
3. If GPT prompt behavior changed, update this file and prompt docs sections.

## Commit Style

- Use concise English commit messages.
- Group related changes in one commit.
- Avoid mixing unrelated refactors with behavior changes.
