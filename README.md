# wowschat-translator

**[日本語](README.ja.md)**

A local tool that translates battle chat in World of Warships in real time.
A headless port of [WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator), with a few extra tricks.

As an optional extension not present in the original project, GPT and Claude modes enable more customizable translation behavior when tuned carefully.
GPT and Claude translation are available as advanced optional modes for users who understand prompt/model tuning and API cost trade-offs.
The default and recommended engine remains DeepL, same as the original project.

## Highlights

- DeepL-first operation for stable and simple setup
- Optional GPT / Claude engines (not in the original project) for prompt-driven customization
- Configurable passthrough/glossary/expand controls for game-specific terms

## How it works

TTaro Chat Mod sends chat messages to `http://localhost:5000/wowschat/?text=...`,
and wowschat-translator translates them with the selected engine (DeepL by default, GPT/Claude optional) and returns the result.

```
WoWs (TTaro Chat Mod)
  └─ GET http://localhost:5000/wowschat/?text=[message]
        └─ wowschat-translator
              ├─ DeepL API (default)
              ├─ OpenAI Responses API (optional GPT)
              └─ Anthropic Messages API (optional Claude)
```

## Requirements

- [TTaro Chat Mod](https://github.com/AndrewTaro/TTaroChat)
- An API key for the translation engine you choose:
  - [DeepL API key](https://www.deepl.com/pro-api) (free plan is fine) — for `translation_engine: deepl` (default)
  - [OpenAI API key](https://platform.openai.com/api-keys) — for `translation_engine: gpt`
  - [Anthropic API key](https://console.anthropic.com/settings/keys) — for `translation_engine: claude`

## Acknowledgements

Special thanks to TTaro, the author of [TTaroChat](https://github.com/AndrewTaro/TTaroChat)
and [WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator).

## Installation

Download the release package from [Releases](../../releases).

- Recommended: `wowschat-translator-windows-amd64.zip` (includes exe and `config.yaml.example`)
- Also available: standalone `wowschat-translator-windows-amd64.exe`

### Building from source

```bash
GOOS=windows GOARCH=amd64 go build -o wowschat-translator.exe ./cmd/wowschat-translator/
```

## Configuration

### Config file (recommended)

If `config.yaml` is missing, the app auto-creates a default config file on startup.
You can also generate it manually:

```
wowschat-translator.exe --init-config
```

The generated file is created next to the executable (or at `--config` path when provided).

Minimal example:

```yaml
api_key: "your-deepl-api-key"
target_lang: "JA"
output_format: "({DetectedSourceLanguage}) {TranslatedText}"
trace_log_file: "logs/trace.jsonl"
```

`trace_log_file` is optional. If set, translator trace events are appended as JSON Lines.
Relative paths are resolved from the runtime working directory.

### Environment variables

```
WOWSCHAT_API_KEY=your-deepl-api-key
WOWSCHAT_TARGET_LANG=JA
WOWSCHAT_OUTPUT_FORMAT=({DetectedSourceLanguage}) {TranslatedText}
```

### Command-line flags

```
wowschat-translator.exe --api-key=your-deepl-api-key --target-lang=JA --output-format="({DetectedSourceLanguage}) {TranslatedText}"
```

**Priority:** command-line flags > environment variables > config file

### Output format tags

You can customize translated output with `output_format`.
`output_format` is optional; if omitted, output stays compatible with WoWSChatTranslator.

Default:

```
({DetectedSourceLanguage}) {TranslatedText}
```

Available tags:

| Tag | Description |
|-----|-------------|
| `{DetectedSourceLanguage}` | Source language detected by the engine (uppercased, e.g. `EN`) |
| `{TargetLanguage}` | Requested target language (e.g. `JA`) |
| `{SourceText}` | Original input text |
| `{TranslatedText}` | Translated text |

**Tip:** If translations sometimes lose context, showing both the original and translated text can help:

```
output_format: "({DetectedSourceLanguage}) {SourceText}\n({TargetLanguage}) {TranslatedText}"
```

### Optional: GPT translation (advanced)

`translation_engine: gpt` is intended for advanced users who want to tune model/prompt behavior.

Notes:

- You need your own OpenAI API key.
- You are responsible for model/temperature tuning and API usage cost.

Config file example:

```yaml
translation_engine: "gpt"
openai_api_key: "your-openai-api-key"
openai_model: "gpt-5.4-mini"
openai_temperature: 0.2
openai_prompt_file: "prompts/my_gpt_system_prompt.txt" # optional

passthrough:
      - gg
      - /\b[A-Z]{2,}\b/

expand:
      cap: capture
      torps: torpedoes

glossary:
      AP: armor-piercing
      DD: destroyer
```

Environment variables:

```
WOWSCHAT_TRANSLATION_ENGINE=gpt
WOWSCHAT_OPENAI_API_KEY=your-openai-api-key
WOWSCHAT_OPENAI_MODEL=gpt-5.4-mini
WOWSCHAT_OPENAI_TEMPERATURE=0.2
WOWSCHAT_OPENAI_PROMPT_FILE=prompts/my_gpt_system_prompt.txt
```

Command-line flags:

```
wowschat-translator.exe --translation-engine=gpt --openai-api-key=your-openai-api-key --openai-model=gpt-5.4-mini --openai-temperature=0.2 --openai-prompt-file=prompts/my_gpt_system_prompt.txt
```

Prompt placeholders (external prompt file only):

- `{{PASSTHROUGH}}`
- `{{GLOSSARY}}`

If an external prompt file is loaded, these placeholders are replaced in-place.
If placeholders are omitted in that external file, passthrough/glossary are not auto-appended.
When no external prompt file is used (embedded default prompt), passthrough/glossary are appended automatically for backward compatibility.

`expand` expands abbreviations into full words before translation (e.g. `cap` → `capture`). This uses word-boundary matching, so it won't match inside longer words. Unlike glossary (which hints the LLM via prompt), expand rewrites the input text directly, giving the LLM a grammatically clearer sentence to work with.

### Optional: Claude translation (advanced)

`translation_engine: claude` is intended for advanced users who want to tune model/prompt behavior using Anthropic's Claude.

Notes:

- You need your own Anthropic API key.
- You are responsible for model/temperature tuning and API usage cost.

Config file example:

```yaml
translation_engine: "claude"
anthropic_api_key: "your-anthropic-api-key"
anthropic_model: "claude-haiku-4-5-20251001"
anthropic_temperature: 0.2
anthropic_prompt_file: "prompts/my_claude_system_prompt.txt" # optional

passthrough:
      - gg
      - /\b[A-Z]{2,}\b/

expand:
      cap: capture
      torps: torpedoes

glossary:
      AP: armor-piercing
      DD: destroyer
```

Environment variables:

```
WOWSCHAT_TRANSLATION_ENGINE=claude
WOWSCHAT_ANTHROPIC_API_KEY=your-anthropic-api-key
WOWSCHAT_ANTHROPIC_MODEL=claude-haiku-4-5-20251001
WOWSCHAT_ANTHROPIC_TEMPERATURE=0.2
WOWSCHAT_ANTHROPIC_PROMPT_FILE=prompts/my_claude_system_prompt.txt
```

Command-line flags:

```
wowschat-translator.exe --translation-engine=claude --anthropic-api-key=your-anthropic-api-key --anthropic-model=claude-haiku-4-5-20251001 --anthropic-temperature=0.2 --anthropic-prompt-file=prompts/my_claude_system_prompt.txt
```

Prompt placeholders work the same as GPT mode (`{{PASSTHROUGH}}`, `{{GLOSSARY}}`).

### Optional: Gemini translation (advanced)

`translation_engine: gemini` is intended for advanced users who want to tune model/prompt behavior using Google's Gemini.

Notes:

- You need your own Google AI (Gemini) API key.
- You are responsible for model/temperature tuning and API usage cost.

Config file example:

```yaml
translation_engine: "gemini"
gemini_api_key: "your-gemini-api-key"
gemini_model: "gemini-2.5-flash"
gemini_temperature: 0.2
gemini_prompt_file: "prompts/my_gemini_system_prompt.txt" # optional

passthrough:
      - gg
      - /\b[A-Z]{2,}\b/

expand:
      cap: capture
      torps: torpedoes

glossary:
      AP: armor-piercing
      DD: destroyer
```

Environment variables:

```
WOWSCHAT_TRANSLATION_ENGINE=gemini
WOWSCHAT_GEMINI_API_KEY=your-gemini-api-key
WOWSCHAT_GEMINI_MODEL=gemini-2.5-flash
WOWSCHAT_GEMINI_TEMPERATURE=0.2
WOWSCHAT_GEMINI_PROMPT_FILE=prompts/my_gemini_system_prompt.txt
```

Command-line flags:

```
wowschat-translator.exe --translation-engine=gemini --gemini-api-key=your-gemini-api-key --gemini-model=gemini-2.5-flash --gemini-temperature=0.2 --gemini-prompt-file=prompts/my_gemini_system_prompt.txt
```

Prompt placeholders work the same as GPT mode (`{{PASSTHROUGH}}`, `{{GLOSSARY}}`).

### Target language codes

Specify a language code (case-insensitive). The format follows the DeepL convention; GPT, Claude, and Gemini engines interpret these codes via prompt.

| Code | Language |
|------|----------|
| `JA` | Japanese |
| `EN-US` | English (American) |
| `EN-GB` | English (British) |
| `ZH-HANS` | Chinese (Simplified) |
| `KO` | Korean |
| `DE` | German |
| `FR` | French |
| `RU` | Russian |

For a full list of language codes, see the [DeepL documentation](https://developers.deepl.com/docs/resources/supported-languages).

## Usage

### Interactive mode

```
wowschat-translator.exe
```

If a config file is found, it is loaded automatically and translation starts immediately.
Press `Ctrl+C` to stop.

### Running as a Windows service

Run in an administrator command prompt.

```
# Install the service
wowschat-translator.exe install

# Start the service
wowschat-translator.exe start

# Stop the service
wowschat-translator.exe stop

# Uninstall the service
wowschat-translator.exe uninstall
```

Once installed, you can also manage it from Windows Services (`services.msc`).
When running as a service, `config.yaml` must be in the **same directory as the exe**.

## DeepL API key notes

- Free plan keys end with `:fx` (e.g. `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:fx`)
- Keys ending with `:fx` automatically use the free endpoint (`api-free.deepl.com`)
- Pro plan keys work as-is
