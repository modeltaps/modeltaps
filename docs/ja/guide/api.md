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
「API キー」ページで任意の API キーの「テストコマンド」を開くと、Modeltaps が実際のキー、サイトアドレス、利用可能なモデルを組み合わせて、そのまま実行できる `curl` を生成します。ターミナルにコピーして動作を確認できます。
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

対話のほかに、`/v1` 配下ではテキスト補完、Responses、ベクトル化（embeddings）、リランキング（rerank）、コンテンツモデレーション（moderations）、画像生成 / 編集 / バリエーション、音声合成 / 文字起こし / 翻訳、およびリアルタイム（realtime）の各 API を提供しています。リクエストとレスポンスの構造は OpenAI 公式と同じです。

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

## コンソールの API カタログ

サイドバーの「API」グループには、モダリティごとのページが並びます。`/panel/api` は概要ページで、各モダリティのカードから「API ドキュメント」または「コンソール」に進めます。API ドキュメントページの内容は本ページと同じですが、アドレスは実際に利用しているサイトアドレスに置き換わっています。

| モダリティ | API ドキュメント | コンソール | 対象エンドポイント |
| --- | --- | --- | --- |
| チャット | `/panel/api/docs/chat` | `/panel/api/chat` | `/v1/chat/completions`、`/v1/responses`、`/claude/v1/messages`、`/gemini/v1beta/models/{model}:generateContent` |
| 画像 | `/panel/api/docs/image` | `/panel/api/image` | `/v1/images/generations`、`/v1/images/edits`、`/v1/images/variations`、および Recraft と Midjourney の拡張エンドポイント |
| 音声 | `/panel/api/docs/speech` | `/panel/api/speech` | `/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`、および `/v1/realtime` |
| 動画 | `/panel/api/docs/video` | —（未公開） | Kling のテキスト / 画像からの動画生成とタスク照会、および Gemini Veo の長時間タスク |

各 API ドキュメントページには「エンドポイント」「リクエスト例」「利用可能なモデル」と完全なドキュメントへのリンクがあります。例に含まれるキーはプレースホルダー `sk-YOUR_TOKEN` です。「API キー」ページで作成したキーに置き換えればそのまま実行できます。

### コンソール

「チャット」「画像」「音声」の 3 ページがそのままコンソールで、本サイトの `/v1` を直接呼び出します。サンプルカードか下部の入力バーから始め、モデルとパラメーターは入力バー上のチップで選びます。チャットはストリーミング出力でいつでも停止でき、ターンごとに最初のトークンまでの遅延・所要時間・使用量を表示します。画像は生成結果のプレビューとダウンロードができ、音声は「音声合成」「音声認識」の 2 つのセグメントに分かれ、合成音声の試聴とダウンロード、音声ファイルのアップロードまたは録音による文字起こしに対応します。各ページで送信したリクエストと等価なコードを確認できるので、そのまま自分のプログラムに取り込めます。

モデル選択はモデルチップに付くメニューで、検索・機能での絞り込み・プロバイダー別の分類に対応し、その機能で利用可能なモデルだけを表示します。

- コンソールはシステムが自動作成する専用キーを使います。表示もコピーもされず、**使用量と料金はあなたの個人アカウントに計上**され、手動で作成した API キーとは別に課金されます。
- 個人コンテキストでのみ利用でき、組織に切り替えるとページに案内と移動先が表示されます（組織クォータでの試用は別途対応）。
- その機能に利用可能なモデルがない場合もページは閲覧でき、理由と対応先を表示し、リクエストは送信しません。
- 管理者はシステム設定で `builtin_chat_enabled` をオフにすると、サイト全体でコンソールを無効にできます。
- 旧 Playground のアドレス（`/playground`、`/panel/playground`）は `/panel/api/chat` に自動で転送されます。

#### 比較

サイドバーの「API」グループにある「比較」（`/panel/api/compare`）では、2〜4 個のチャットモデルを列として並べます。右上の「列を追加」で最大 4 列まで増やせ、各列の見出しでモデルを選び、温度・最大 tokens・思考の深さを上書きできます。下部の共有入力バーで「入力を同期」がオンなら 1 つのメッセージを全列に同時送信し、オフなら選択中の列だけに送ります。各列は個別に停止できます。「コードを見る」では列を切り替えて、上書きを反映した各列の等価なリクエストを確認できます。

Midjourney と非同期タスクの実行記録はカタログページには含まれません。「ログ」ページの「Midjourney」「非同期タスク」タブで確認してください。

## モデルとグループについて

- 呼び出せるモデルは、API キーで選択したチャネルグループと、その API キーでモデル制限を有効にしているかどうかによって決まります。利用可能な一覧は「ダッシュボード」の「現在利用可能なモデル」で確認できるほか、上記の各プロトコルの models API でも照会できます。
- Claude / Gemini プロトコルのリクエストは、対応する種類のチャネルにしかルーティングできません。これらのプロトコルでモデルが見つからない場合は、現在のグループにその種類のチャネルが存在しないということなので、OpenAI 互換 API をご利用ください。
- リクエストが失敗した場合は、[利用状況とログ](/ja/guide/usage) ページで API キーと時刻から該当するレコードを特定できます。明細には終了理由と課金の内訳が表示されます。

## 関連

- [推論設定](/ja/guide/reasoning)：推論モデルの思考の挙動を制御する
- [特殊な呼び出し](/ja/guide/special)：一般的でないシナリオでの呼び出し方法
- [API キーとクォータ](/ja/guide/tokens)：クォータ、有効期限、モデル制限、IP ホワイトリスト
- [よくある質問](/ja/guide/faq)：接続エラーのトラブルシューティング
