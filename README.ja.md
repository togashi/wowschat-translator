# wowschat

**[English](README.md)**

World of Warships のゲーム内チャットを DeepL で翻訳するサービス。
[WoWSChatTranslator](https://github.com/AndrewTaro/WoWSChatTranslator) の GUI なし移植版。

## 仕組み

TTaro Chat Mod がチャットメッセージを `http://localhost:5000/wowschat/?text=...` へ送信し、
wowschat がそれを DeepL API で翻訳してレスポンスとして返す。

```
WoWs (TTaro Chat Mod)
  └─ GET http://localhost:5000/wowschat/?text=[メッセージ]
        └─ wowschat
              └─ DeepL API
```

## 必要なもの

- [DeepL API キー](https://www.deepl.com/ja/pro-api)（無料プランで可）
- [TTaro Chat Mod](https://github.com/TTaro/TTaroChat)

## インストール

[Releases](../../releases) から `wowschat.exe` をダウンロードして任意の場所に置く。

### ビルドする場合

```bash
GOOS=windows GOARCH=amd64 go build -o wowschat.exe ./cmd/wowschat/
```

## 設定

### 設定ファイル（推奨）

`wowschat.exe` と同じディレクトリに `config.yaml` を作成する。

```yaml
api_key: "your-deepl-api-key"
target_lang: "JA"
```

`config.yaml.example` をコピーして編集すると楽。

### 環境変数

```
WOWSCHAT_API_KEY=your-deepl-api-key
WOWSCHAT_TARGET_LANG=JA
```

### コマンドライン引数

```
wowschat.exe --api-key=your-deepl-api-key --target-lang=JA
```

**優先順位:** コマンドライン引数 > 環境変数 > 設定ファイル

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
wowschat.exe
```

設定ファイルが見つかれば自動で読み込まれ、すぐに翻訳を受け付け始める。
停止するには `Ctrl+C`。

### Windows サービスとして動作させる

管理者権限のコマンドプロンプトで実行する。

```
# サービス登録
wowschat.exe install

# サービス開始
wowschat.exe start

# サービス停止
wowschat.exe stop

# サービス削除
wowschat.exe uninstall
```

サービス登録後は Windows のサービス管理（`services.msc`）からも操作できる。
サービスとして動作する場合、`config.yaml` は **exe と同じディレクトリ** に置く必要がある。

## DeepL API キーについて

- 無料プランのキーは末尾が `:fx`（例: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:fx`）
- 末尾が `:fx` のキーは自動的に無料エンドポイント（`api-free.deepl.com`）を使用する
- 有料プランのキーはそのまま指定すれば良い
