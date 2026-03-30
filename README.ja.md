# wowschat-translator

**[English](README.md)**

World of Warships のゲーム内チャットを DeepL で翻訳するサービス。
[WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator) の GUI なし移植版。

## 仕組み

TTaro Chat Mod がチャットメッセージを `http://localhost:5000/wowschat/?text=...` へ送信し、
wowschat-translator がそれを DeepL API で翻訳してレスポンスとして返す。

```
WoWs (TTaro Chat Mod)
  └─ GET http://localhost:5000/wowschat/?text=[メッセージ]
        └─ wowschat-translator
              └─ DeepL API
```

## 必要なもの

- [DeepL API キー](https://www.deepl.com/ja/pro-api)（無料プランで可）
- [TTaro Chat Mod](https://github.com/AndrewTaro/TTaroChat)

## 謝辞

[TTaroChat](https://github.com/AndrewTaro/TTaroChat) と
[WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator) の作者である
TTaro 氏に感謝します。

## インストール

[Releases](../../releases) から `wowschat-translator.exe` をダウンロードして任意の場所に置く。

### ビルドする場合

```bash
GOOS=windows GOARCH=amd64 go build -o wowschat-translator.exe ./cmd/wowschat-translator/
```

## 設定

### 設定ファイル（推奨）

`wowschat-translator.exe` と同じディレクトリに `config.yaml` を作成する。

```yaml
api_key: "your-deepl-api-key"
target_lang: "JA"
output_format: "({DetectedSourceLanguage}) {TranslatedText}"
trace_log_file: "logs/trace.jsonl"
```

`config.yaml.example` をコピーして編集すると楽。

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
