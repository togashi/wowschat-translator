# wowschat

**[日本語](README.ja.md)**

A service that translates in-game chat in World of Warships using DeepL.
A headless port of [WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator).

## How it works

TTaro Chat Mod sends chat messages to `http://localhost:5000/wowschat/?text=...`,
and wowschat translates them via the DeepL API and returns the result.

```
WoWs (TTaro Chat Mod)
  └─ GET http://localhost:5000/wowschat/?text=[message]
        └─ wowschat
              └─ DeepL API
```

## Requirements

- [DeepL API key](https://www.deepl.com/pro-api) (free plan is fine)
- [TTaro Chat Mod](https://github.com/TTaro/TTaroChat)

## Installation

Download `wowschat.exe` from [Releases](../../releases) and place it anywhere you like.

### Building from source

```bash
GOOS=windows GOARCH=amd64 go build -o wowschat.exe ./cmd/wowschat/
```

## Configuration

### Config file (recommended)

Create a `config.yaml` in the same directory as `wowschat.exe`.

```yaml
api_key: "your-deepl-api-key"
target_lang: "JA"
```

You can copy `config.yaml.example` and edit it.

### Environment variables

```
WOWSCHAT_API_KEY=your-deepl-api-key
WOWSCHAT_TARGET_LANG=JA
```

### Command-line flags

```
wowschat.exe --api-key=your-deepl-api-key --target-lang=JA
```

**Priority:** command-line flags > environment variables > config file

### Target language codes

Specify a language code supported by DeepL (case-insensitive).

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

See the [DeepL documentation](https://developers.deepl.com/docs/resources/supported-languages) for all language codes.

## Usage

### Interactive mode

```
wowschat.exe
```

If a config file is found, it is loaded automatically and translation starts immediately.
Press `Ctrl+C` to stop.

### Running as a Windows service

Run in an administrator command prompt.

```
# Install the service
wowschat.exe install

# Start the service
wowschat.exe start

# Stop the service
wowschat.exe stop

# Uninstall the service
wowschat.exe uninstall
```

Once installed, you can also manage it from Windows Services (`services.msc`).
When running as a service, `config.yaml` must be in the **same directory as the exe**.

## DeepL API key notes

- Free plan keys end with `:fx` (e.g. `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:fx`)
- Keys ending with `:fx` automatically use the free endpoint (`api-free.deepl.com`)
- Pro plan keys work as-is
