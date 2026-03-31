# wowschat-translator

**[English](README.md)**

World of Warships の戦闘中チャットをリアルタイムに翻訳するローカルツール。
[WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator) の GUI なし移植版 + α。

本家にはない拡張機能として、GPT / Claude モードでは調整次第でより柔軟な翻訳挙動を実現できる。
GPT / Claude 翻訳は、プロンプトやモデル調整、API コスト管理を理解している人向けの任意オプション。
既定・推奨エンジンは本家と同じ DeepL。

## ハイライト

- DeepL を軸にした安定・シンプルな運用
- 本家にはない任意機能としての GPT / Claude エンジンによるプロンプト駆動の調整
- ゲーム用語向けの passthrough / glossary / expand カスタマイズ

## 仕組み

TTaro Chat Mod がチャットメッセージを `http://localhost:5000/wowschat/?text=...` へ送信し、
wowschat-translator が選択されたエンジン（既定は DeepL、任意で GPT / Claude）で翻訳してレスポンスとして返す。

```
WoWs (TTaro Chat Mod)
  └─ GET http://localhost:5000/wowschat/?text=[メッセージ]
        └─ wowschat-translator
              ├─ DeepL API（既定）
              ├─ OpenAI Responses API（任意: GPT）
              └─ Anthropic Messages API（任意: Claude）
```

## 必要なもの

### 必須

- [DeepL API キー](https://www.deepl.com/ja/pro-api)（無料プランで可）
- [TTaro Chat Mod](https://github.com/AndrewTaro/TTaroChat)

### 任意（GPT / Claude 翻訳を使う場合のみ）

- [OpenAI API キー](https://platform.openai.com/api-keys)（GPT）
- [Anthropic API キー](https://console.anthropic.com/settings/keys)（Claude）

## 謝辞

[TTaroChat](https://github.com/AndrewTaro/TTaroChat) と
[WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator) の作者である
TTaro 氏に感謝します。

## インストール

[Releases](../../releases) から配布物をダウンロードする。

- 推奨: `wowschat-translator-windows-amd64.zip`（exe と `config.yaml.example` を同梱）
- 単体版: `wowschat-translator-windows-amd64.exe`

### ビルドする場合

```bash
GOOS=windows GOARCH=amd64 go build -o wowschat-translator.exe ./cmd/wowschat-translator/
```

## 設定

### 設定ファイル（推奨）

`config.yaml` がない場合、起動時に既定の設定ファイルを自動生成する。
手動で生成したい場合は以下を実行する。

```
wowschat-translator.exe --init-config
```

生成先は exe と同じディレクトリ（`--config` 指定時はそのパス）。

最小設定例:

```yaml
api_key: "your-deepl-api-key"
target_lang: "JA"
output_format: "({DetectedSourceLanguage}) {TranslatedText}"
trace_log_file: "logs/trace.jsonl"
```

`trace_log_file` は任意項目。設定した場合は translator のトレースイベントを JSON Lines で追記する。
相対パスは実行時のカレントディレクトリ基準で解決される。

### 環境変数

```
WOWSCHAT_API_KEY=your-deepl-api-key
WOWSCHAT_TARGET_LANG=JA
WOWSCHAT_OUTPUT_FORMAT=({DetectedSourceLanguage}) {TranslatedText}
```

### コマンドライン引数

```
wowschat-translator.exe --api-key=your-deepl-api-key --target-lang=JA --output-format="({DetectedSourceLanguage}) {TranslatedText}"
```

**優先順位:** コマンドライン引数 > 環境変数 > 設定ファイル

### 出力フォーマットタグ

`output_format` で翻訳結果の出力形式をカスタマイズできる。
`output_format` は省略可能で、省略した場合は WoWSChatTranslator 互換の出力形式になる。

デフォルト値:

```
({DetectedSourceLanguage}) {TranslatedText}
```

使用可能なタグ:

| タグ | 説明 |
|------|------|
| `{DetectedSourceLanguage}` | DeepL が検出した元言語（大文字化。例: `EN`） |
| `{TargetLanguage}` | 指定した翻訳先言語（例: `JA`） |
| `{SourceText}` | 元の入力テキスト |
| `{TranslatedText}` | DeepL の翻訳結果テキスト |

**Tips:** 翻訳だけだと文脈が分かりにくいことがある場合、原文と訳文を両方表示する設定が便利:

```
output_format: "({DetectedSourceLanguage}) {SourceText}\n({TargetLanguage}) {TranslatedText}"
```

### 任意機能: GPT 翻訳（上級者向け）

`translation_engine: gpt` は、モデルやプロンプトを自分で調整したい上級者向けの設定。

注意点:

- OpenAI API キーが必要。
- モデル/temperature の調整と API 利用コスト管理は利用者側の責任。
- 手軽さと保守性を重視するなら DeepL のままが安全。

設定ファイル例:

```yaml
translation_engine: "gpt"
openai_api_key: "your-openai-api-key"
openai_model: "gpt-5.4-mini"
openai_temperature: 0.2
openai_prompt_file: "prompts/my_gpt_system_prompt.txt" # 任意

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

環境変数:

```
WOWSCHAT_TRANSLATION_ENGINE=gpt
WOWSCHAT_OPENAI_API_KEY=your-openai-api-key
WOWSCHAT_OPENAI_MODEL=gpt-5.4-mini
WOWSCHAT_OPENAI_TEMPERATURE=0.2
WOWSCHAT_OPENAI_PROMPT_FILE=prompts/my_gpt_system_prompt.txt
```

コマンドライン引数:

```
wowschat-translator.exe --translation-engine=gpt --openai-api-key=your-openai-api-key --openai-model=gpt-5.4-mini --openai-temperature=0.2 --openai-prompt-file=prompts/my_gpt_system_prompt.txt
```

プロンプトプレースホルダー（外部プロンプトファイル指定時のみ有効）:

- `{{PASSTHROUGH}}`
- `{{GLOSSARY}}`

外部プロンプトファイルを読み込んだ場合、これらのプレースホルダー位置に展開される。
外部プロンプト内にプレースホルダーがない場合、passthrough/glossary は自動追記されない。
外部プロンプトを使わず埋め込み既定プロンプトを使う場合は、後方互換のため自動追記される。

`expand` は略語を翻訳前にフルスペルに展開する（例: `cap` → `capture`）。単語境界でマッチするため、長い単語の一部に誤マッチしない。glossary（プロンプト経由の翻訳指示）と異なり、入力テキストを直接書き換えるため、LLM が文法的に正しい文として解釈しやすくなる。

### 任意機能: Claude 翻訳（上級者向け）

`translation_engine: claude` は、Anthropic の Claude を使ってモデルやプロンプトを自分で調整したい上級者向けの設定。

注意点:

- Anthropic API キーが必要。
- モデル/temperature の調整と API 利用コスト管理は利用者側の責任。
- 手軽さと保守性を重視するなら DeepL のままが安全。

設定ファイル例:

```yaml
translation_engine: "claude"
anthropic_api_key: "your-anthropic-api-key"
anthropic_model: "claude-haiku-4-5-20251001"
anthropic_temperature: 0.2
anthropic_prompt_file: "prompts/my_claude_system_prompt.txt" # 任意

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

環境変数:

```
WOWSCHAT_TRANSLATION_ENGINE=claude
WOWSCHAT_ANTHROPIC_API_KEY=your-anthropic-api-key
WOWSCHAT_ANTHROPIC_MODEL=claude-haiku-4-5-20251001
WOWSCHAT_ANTHROPIC_TEMPERATURE=0.2
WOWSCHAT_ANTHROPIC_PROMPT_FILE=prompts/my_claude_system_prompt.txt
```

コマンドライン引数:

```
wowschat-translator.exe --translation-engine=claude --anthropic-api-key=your-anthropic-api-key --anthropic-model=claude-haiku-4-5-20251001 --anthropic-temperature=0.2 --anthropic-prompt-file=prompts/my_claude_system_prompt.txt
```

プロンプトプレースホルダーは GPT モードと同様（`{{PASSTHROUGH}}`、`{{GLOSSARY}}`）。

### 翻訳先言語コード

DeepL がサポートする言語コードを指定する（大文字小文字は問わない）。

| コード | 言語 |
|--------|------|
| `JA` | 日本語 |
| `EN-US` | 英語（アメリカ） |
| `EN-GB` | 英語（イギリス） |
| `ZH-HANS` | 中国語（簡体字） |
| `KO` | 韓国語 |
| `DE` | ドイツ語 |
| `FR` | フランス語 |
| `RU` | ロシア語 |

全言語コードは [DeepL ドキュメント](https://developers.deepl.com/docs/resources/supported-languages) を参照。

## 使い方

### 通常起動（対話モード）

```
wowschat-translator.exe
```

設定ファイルが見つかれば自動で読み込まれ、すぐに翻訳を受け付け始める。
停止するには `Ctrl+C`。

### Windows サービスとして動作させる

管理者権限のコマンドプロンプトで実行する。

```
# サービス登録
wowschat-translator.exe install

# サービス開始
wowschat-translator.exe start

# サービス停止
wowschat-translator.exe stop

# サービス削除
wowschat-translator.exe uninstall
```

サービス登録後は Windows のサービス管理（`services.msc`）からも操作できる。
サービスとして動作する場合、`config.yaml` は **exe と同じディレクトリ** に置く必要がある。

## DeepL API キーについて

- 無料プランのキーは末尾が `:fx`（例: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:fx`）
- 末尾が `:fx` のキーは自動的に無料エンドポイント（`api-free.deepl.com`）を使用する
- 有料プランのキーはそのまま指定すれば良い
