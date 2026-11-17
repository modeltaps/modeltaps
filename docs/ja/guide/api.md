---
title: "API 接続"
layout: doc
outline: deep
lastUpdated: true
---

# API 接続

Modeltaps は OpenAI、Claude、Gemini の 3 種類の互換 API を同時に提供します。既存のアプリケーションはコードを変更する必要はなく、接続先アドレスを Modeltaps のサイトアドレスに、API Key を [API キー](/ja/guide/tokens) ページで作成したキーに差し替えるだけで利用できます。

以下の例に出てくる `https://your-modeltaps-domain.com` は実際に利用しているサイトアドレスに、`sk-ここを自分のキーに置き換えてください` はご自身の API キーに置き換えてください。3 種類のプロトコルはいずれも同じ API キーを使用します。

::: tip まず疎通を確認してから接続する
「API キー」ページで任意の API キーの「テストコマンド」を開くと、Modeltaps がキー、サイトアドレス、利用可能なモデルを組み合わせて、そのまま実行できる `curl` を生成します。ターミナルにコピーして動作を確認できます。キーは一度しか表示されないため、作成・再発行した直後でないキーは、先にダイアログに貼り付けてください。
:::

## OpenAI 互換 API

利用方法は [OpenAI API](https://platform.openai.com/docs/api-reference/introduction) と同じで、接続先アドレスはサイトのルートアドレスです。

::: warning API Base の書き方
クライアントによって期待する API Base の形式が異なります。接続に失敗する場合は次の順に試してください。

- `https://your-modeltaps-domain.com`
- `https://your-modeltaps-domain.com/v1`
- `https://your-modeltaps-domain.com/v1/chat/completions`
  :::

### 利用例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/v1/chat/completions \
    --header 'Authorization: Bearer sk-ここを自分のキーに置き換えてください' \
    -H "Content-Type: application/json" \
    --data '{
      "model": "gpt-5-mini",
      "messages": [
          {
              "role": "user",
              "content": "hi~"
          }
      ]
  }'
```

### 利用できる API

対話のほかに、`/v1` 配下ではテキスト補完、Responses、ベクトル化（embeddings）、リランキング（rerank）、コンテンツモデレーション（moderations）、画像生成 / 編集 / バリエーション、音声合成 / 文字起こし / 翻訳、およびリアルタイム（realtime）の各 API を提供しています。リクエストとレスポンスの構造は OpenAI 公式と同じです。意思決定モデルは `/v1/decisions` で呼び出します。詳しくは[意思決定モデル](/ja/guide/decisions)を参照してください。

現在の API キーで利用できるモデルを照会します。

```bash
curl https://your-modeltaps-domain.com/v1/models \
  --header 'Authorization: Bearer sk-ここを自分のキーに置き換えてください'
```

## Claude 互換 API

利用方法は [Claude API](https://docs.anthropic.com/en/api/messages) と同じで、接続先アドレスは `https://your-modeltaps-domain.com/claude`、認証には `x-api-key` ヘッダーを使用します。

### 利用例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/claude/v1/messages \
    -H "Content-Type: application/json" \
    -H "x-api-key: sk-ここを自分のキーに置き換えてください" \
    -H "anthropic-version: 2023-06-01" \
    --data '{
  "model": "claude-sonnet-4-5",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "hi~"
    }
  ]
}'
```

このプロトコルで利用できるモデルを照会します。

```bash
curl https://your-modeltaps-domain.com/claude/v1/models \
  -H "x-api-key: sk-ここを自分のキーに置き換えてください"
```

## Gemini 互換 API

接続先アドレスは `https://your-modeltaps-domain.com/gemini`、認証には `x-goog-api-key` ヘッダーを使用します。パス内のバージョン部分（`v1beta` など）は公式と同じです。

### 利用例

```bash
curl --request POST \
  --url https://your-modeltaps-domain.com/gemini/v1beta/models/gemini-2.5-pro:generateContent \
  --header 'Content-Type: application/json' \
  --header 'x-goog-api-key: sk-ここを自分のキーに置き換えてください' \
  --data '{
	"contents": [
		{
			"role": "user",
			"parts": [
				{
					"text": "hi~"
				}
			]
		}
	]
}'
```

このプロトコルで利用できるモデルを照会します。

```bash
curl https://your-modeltaps-domain.com/gemini/v1beta/models \
  -H "x-goog-api-key: sk-ここを自分のキーに置き換えてください"
```

## コーディングエージェント

コーディングエージェントには専用の接続先アドレスがあります。パスはどのエージェントが呼び出しているかを表し、プロトコルはその後ろのパスで決まります。認証、ルーティング、課金、エラーは上記の API と同じです。違うのはモデル一覧だけで、各アドレスはそのエージェントが読める形式で返すため、エージェントのモデル選択画面に API キーで呼び出せるモデルが表示されます。

| 接続先アドレス | プロトコル | 対象 |
| --- | --- | --- |
| `https://your-modeltaps-domain.com/claude-code` | Anthropic Messages：`/v1/messages`、`/v1/messages/count_tokens`、`/v1/models` | Claude Code と Claude Agent SDK |
| `https://your-modeltaps-domain.com/codex/v1` | OpenAI：`/responses`、`/chat/completions` など `/v1` の API | Codex CLI |
| `https://your-modeltaps-domain.com/coding-agent/v1` | OpenAI（`/v1` と同じ） | その他の OpenAI 互換コーディングエージェント |
| `https://your-modeltaps-domain.com/coding-agent` | Anthropic Messages（`/claude` と同じ） | その他の Anthropic 互換コーディングエージェント |

利用ログには各リクエストを送ったエージェントが記録されます。接続先アドレスで判定し、それ以外のアドレスでは User-Agent から識別します（Claude Code、Codex、OpenCode、Gemini CLI）。組織ではツール別の利用状況を確認できます。[組織](/ja/guide/organization#利用状況) を参照してください。

### Claude Code

`~/.claude/settings.json` にゲートウェイを設定します（同じ変数をシェルで export しても構いません）。

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-modeltaps-domain.com/claude-code",
    "ANTHROPIC_AUTH_TOKEN": "sk-ここを自分のキーに置き換えてください",
    "ANTHROPIC_API_KEY": "",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

- 接続先アドレスに `/v1` は付けません。Claude Code が `/v1/messages` を自動で付け足します。
- `ANTHROPIC_API_KEY` は空にしてください。値があると Claude Code は `ANTHROPIC_AUTH_TOKEN` より先にそちらを使います。
- `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` を設定すると、`/model` に API キーで呼び出せるすべてのチャットモデルが `claude-code/<モデル>` の形式で表示されます。100 万トークン以上の入力を受け付けるモデルは末尾に `[1m]` が付き、Claude Code はそのコンテキスト長を使います。ゲートウェイがプレフィックスとサフィックスを取り除くため、利用状況とログには実際のモデル名が表示されます。画像、音声、埋め込みのモデルは表示されません。
- `cache_control` のキャッシュブレークポイントはそのまま上流に送られ、同じ Claude Code セッションのリクエストは同じチャネルに固定されるため、上流のプロンプトキャッシュが効き続けます。`/context` のトークン数は `/v1/messages/count_tokens` から取得され、これは推定値で課金されません。

### Codex

`~/.codex/config.toml` に provider を追加し、指定した環境変数に API キーを設定します。

```toml
model_provider = "modeltaps"
model = "gpt-5.5"

[model_providers.modeltaps]
name = "Modeltaps"
base_url = "https://your-modeltaps-domain.com/codex/v1"
env_key = "MODELTAPS_API_KEY"
wire_api = "responses"
```

```bash
export MODELTAPS_API_KEY=sk-ここを自分のキーに置き換えてください
```

- `wire_api` は `responses` にしてください。Codex は Responses API のみに対応しています。
- `/codex/v1/models` は Codex が読むモデルカタログの形式で返すため、`/model` に API キーで呼び出せるチャットモデルが表示されます。表示されない場合は、provider に `model_catalog_url = "https://your-modeltaps-domain.com/codex/v1/models"` も追加してください。
- Codex が送る `prompt_cache_key` はそのまま上流に送られ、同じ Codex セッションのリクエストは同じチャネルに固定されます。

### その他のコーディングエージェント

OpenAI 互換のエージェントは `https://your-modeltaps-domain.com/coding-agent/v1`、Anthropic 互換のエージェントは `https://your-modeltaps-domain.com/coding-agent` に向けてください。動作は `/v1`、`/claude` とまったく同じで、リクエストはコーディングエージェントのトラフィックとして集計されます。

## コンソールの API カタログ

サイドバーの「API」グループには、モダリティごとのページが並びます。`/panel/docs` は概要ページで、各モダリティのカードから「API ドキュメント」または「コンソール」に進めます。API ドキュメントページの内容は本ページと同じですが、アドレスは実際に利用しているサイトアドレスに置き換わっています。

| モダリティ | API ドキュメント | コンソール | 対象エンドポイント |
| --- | --- | --- | --- |
| チャット | `/panel/docs/chat` | `/panel/playground/chat` | `/v1/chat/completions`、`/v1/responses`、`/claude/v1/messages`、`/gemini/v1beta/models/{model}:generateContent` |
| 画像 | `/panel/docs/image` | `/panel/playground/image` | `/v1/images/generations`、`/v1/images/edits`、`/v1/images/variations`、および Recraft と Midjourney の拡張エンドポイント |
| 音声 | `/panel/docs/speech` | `/panel/playground/speech` | `/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`、および `/v1/realtime` |
| 動画 | `/panel/docs/video` | —（未公開） | Kling のテキスト / 画像からの動画生成とタスク照会、および Gemini Veo の長時間タスク |

各 API ドキュメントページには「エンドポイント」「リクエスト例」「利用可能なモデル」と完全なドキュメントへのリンクがあります。例に含まれるキーはプレースホルダー `sk-YOUR_TOKEN` です。「API キー」ページで作成したキーに置き換えればそのまま実行できます。

### コンソール

「チャット」「画像」「音声」の 3 ページがそのままコンソールで、本サイトの `/v1` を直接呼び出します。サンプルカードか下部の入力バーから始め、モデルとパラメーターは入力バー上のチップで選びます。チャットはストリーミング出力でいつでも停止でき、ターンごとに最初のトークンまでの遅延・所要時間・使用量を表示します。画像は生成結果のプレビューとダウンロードができ、音声は「音声合成」「音声認識」の 2 つのセグメントに分かれ、合成音声の試聴とダウンロード、音声ファイルのアップロードまたは録音による文字起こしに対応します。各ページで送信したリクエストと等価なコードを確認できるので、そのまま自分のプログラムに取り込めます。

モデル選択はモデルチップに付くメニューで、検索・機能での絞り込み・プロバイダー別の分類に対応し、その機能で利用可能なモデルだけを表示します。

- コンソールはシステムが自動作成する専用キーを使います。表示もコピーもされず、ページは短期間だけ有効な認証情報（有効期間 1 時間、自動で更新）でリクエストを送ります。**使用量と料金はあなたの個人アカウントに計上**され、手動で作成した API キーとは別に課金されます。
- 個人コンテキストでのみ利用でき、組織に切り替えるとページに案内と移動先が表示されます（組織クォータでの試用は別途対応）。
- その機能に利用可能なモデルがない場合もページは閲覧でき、理由と対応先を表示し、リクエストは送信しません。
- 管理者はシステム設定で `builtin_chat_enabled` をオフにすると、サイト全体でコンソールを無効にできます。
- 旧 Playground のアドレス（`/playground`、`/panel/playground`）は `/panel/playground/chat` に自動で転送されます。

#### 比較

サイドバーの「API」グループにある「比較」（`/panel/playground/compare`）では、2〜4 個のチャットモデルを列として並べます。右上の「列を追加」で最大 4 列まで増やせ、各列の見出しでモデルを選び、温度・最大 tokens・思考の深さを上書きできます。下部の共有入力バーで「入力を同期」がオンなら 1 つのメッセージを全列に同時送信し、オフなら選択中の列だけに送ります。各列は個別に停止できます。「コードを見る」では列を切り替えて、上書きを反映した各列の等価なリクエストを確認できます。

Midjourney と非同期タスクの実行記録はカタログページには含まれません。「ログ」ページの「Midjourney」「非同期タスク」タブで確認してください。

## モデルとグループについて

- 呼び出せるモデルは、API キーで選択したチャネルグループと、その API キーでモデル制限を有効にしているかどうかによって決まります。利用可能な一覧は「ダッシュボード」の「現在利用可能なモデル」で確認できるほか、上記の各プロトコルの models API でも照会できます。
- `/claude/v1/messages` のリクエストはあらゆるチャットモデルを呼び出せます。チャネルが Anthropic プロトコルに対応していない場合は、ゲートウェイがリクエストを変換します。どのエンドポイントを呼び出しても、チャネルが対応していれば Claude モデルは Anthropic プロトコルで上流へ送られるため、プロンプトキャッシュが維持されます。
- Gemini プロトコルのリクエストは Gemini 種別のチャネルにしかルーティングできません。Gemini プロトコルでモデルが見つからない場合は、現在のグループにそのチャネルが存在しないということなので、OpenAI 互換 API をご利用ください。
- リクエストが失敗した場合は、[利用状況とログ](/ja/guide/usage) ページで API キーと時刻から該当するレコードを特定できます。明細には終了理由と課金の内訳が表示されます。

## 関連

- [推論設定](/ja/guide/reasoning)：推論モデルの思考の挙動を制御する
- [特殊な呼び出し](/ja/guide/special)：一般的でないシナリオでの呼び出し方法
- [API キーとクォータ](/ja/guide/tokens)：クォータ、有効期限、モデル制限、IP ホワイトリスト
- [よくある質問](/ja/guide/faq)：接続エラーのトラブルシューティング
