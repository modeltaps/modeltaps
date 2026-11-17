---
title: "API 接入"
layout: doc
outline: deep
lastUpdated: true
---

# API 接入

Modeltaps 同時提供 OpenAI、Claude、Gemini 三套相容介面。既有應用無需改代碼，只要把接入地址換成 Modeltaps 的站點地址、把 API Key 換成你在 [API Key](/hk/guide/tokens) 頁面創建的 API Key 即可。

下文示例中的 `https://your-modeltaps-domain.com` 請替換為你實際使用的站點地址，`sk-替換為你的key` 替換為你的 API Key。三套協議都使用同一把 API Key。

::: tip 先跑通再接入
在「API Key」頁面打開某個 API Key 的「測試命令」，Modeltaps 會把你的密鑰、站點地址與可用模型拼成一條可直接運行的 `curl`，複製到終端即可驗證。密鑰只顯示一次，不是剛建立或重設的密鑰，需要先在彈窗裏貼上。
:::

## OpenAI 相容介面

使用方式與 [OpenAI API](https://platform.openai.com/docs/api-reference/introduction) 一致，接入地址為站點根地址。

::: warning API Base 的寫法
不同客戶端對 API Base 的期望格式不同，若連接失敗請依次嘗試：

- `https://your-modeltaps-domain.com`
- `https://your-modeltaps-domain.com/v1`
- `https://your-modeltaps-domain.com/v1/chat/completions`
  :::

### 使用示例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/v1/chat/completions \
    --header 'Authorization: Bearer sk-替換為你的key' \
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

### 可用的介面

除對話之外，`/v1` 下還提供文本補全、Responses、向量化（embeddings）、重排序（rerank）、內容審核（moderations）、圖片生成 / 編輯 / 變體、語音合成 / 轉寫 / 翻譯，以及實時（realtime）介面。請求與響應結構與 OpenAI 官方一致。決策模型透過 `/v1/decisions` 調用，見[決策模型](/hk/guide/decisions)。

查詢當前 API Key 可用的模型：

```bash
curl https://your-modeltaps-domain.com/v1/models \
  --header 'Authorization: Bearer sk-替換為你的key'
```

## Claude 相容介面

使用方式與 [Claude API](https://docs.anthropic.com/en/api/messages) 一致，接入地址為 `https://your-modeltaps-domain.com/claude`，鑑權使用 `x-api-key` 頭。

### 使用示例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/claude/v1/messages \
    -H "Content-Type: application/json" \
    -H "x-api-key: sk-替換為你的key" \
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

查詢該協議下可用的模型：

```bash
curl https://your-modeltaps-domain.com/claude/v1/models \
  -H "x-api-key: sk-替換為你的key"
```

## Gemini 相容介面

接入地址為 `https://your-modeltaps-domain.com/gemini`，鑑權使用 `x-goog-api-key` 頭，路徑中的版本段（如 `v1beta`）與官方保持一致。

### 使用示例

```bash
curl --request POST \
  --url https://your-modeltaps-domain.com/gemini/v1beta/models/gemini-2.5-pro:generateContent \
  --header 'Content-Type: application/json' \
  --header 'x-goog-api-key: sk-替換為你的key' \
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

查詢該協議下可用的模型：

```bash
curl https://your-modeltaps-domain.com/gemini/v1beta/models \
  -H "x-goog-api-key: sk-替換為你的key"
```

## 編程 Agent

編程 Agent 有各自的專屬接入地址。路徑表示是哪個 Agent 在調用，協議仍由後面的路徑決定；鑑權、路由、計費和錯誤都與上面的介面相同。不同的只有模型列表：每個地址按對應 Agent 能識別的格式返回，Agent 的模型選擇器裏就能看到你的 API Key 可調用的模型。

| 接入地址 | 協議 | 適用 |
| --- | --- | --- |
| `https://your-modeltaps-domain.com/claude-code` | Anthropic Messages：`/v1/messages`、`/v1/messages/count_tokens`、`/v1/models` | Claude Code 與 Claude Agent SDK |
| `https://your-modeltaps-domain.com/codex/v1` | OpenAI：`/responses`、`/chat/completions` 以及 `/v1` 下的其他介面 | Codex CLI |
| `https://your-modeltaps-domain.com/coding-agent/v1` | OpenAI，與 `/v1` 相同 | 其他 OpenAI 相容的編程 Agent |
| `https://your-modeltaps-domain.com/coding-agent` | Anthropic Messages，與 `/claude` 相同 | 其他 Anthropic 相容的編程 Agent |

用量日誌會記下每次請求來自哪個 Agent：以接入地址為準，走其他地址時按 User-Agent 識別（Claude Code、Codex、OpenCode、Gemini CLI）。組織可以按工具查看用量，見 [組織](/hk/guide/organization#用量)。

### Claude Code

把網關寫進 `~/.claude/settings.json`（也可以在 shell 裏 export 同樣的變數）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-modeltaps-domain.com/claude-code",
    "ANTHROPIC_AUTH_TOKEN": "sk-替換為你的key",
    "ANTHROPIC_API_KEY": "",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

- 接入地址不帶 `/v1`：Claude Code 會自己補上 `/v1/messages`。
- `ANTHROPIC_API_KEY` 留空。它有值時 Claude Code 會優先用它，而不是 `ANTHROPIC_AUTH_TOKEN`。
- 設定 `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` 後，`/model` 會以 `claude-code/<模型>` 的形式列出你的 Key 可調用的全部對話模型。可接受一百萬 tokens 及以上輸入的模型以 `[1m]` 結尾，Claude Code 會據此使用該上下文長度。網關會去掉前綴和後綴，用量與日誌裏顯示的是真實模型名。生圖、音頻和向量模型不會列出。
- `cache_control` 快取斷點原樣送到上游，同一個 Claude Code 會話的請求固定在同一個渠道上，上游提示快取可以持續命中。`/context` 的 token 數來自 `/v1/messages/count_tokens`，這是估算值，不計費。

### Codex

在 `~/.codex/config.toml` 裏加一個 provider，並把 API Key 放進它指定的環境變數：

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
export MODELTAPS_API_KEY=sk-替換為你的key
```

- `wire_api` 必須是 `responses`：Codex 只支援 Responses API。
- `/codex/v1/models` 按 Codex 讀取的模型目錄格式返回，`/model` 裏就能看到你的 Key 可調用的對話模型。如果 `/model` 裏沒有出現，在 provider 裏再加一行 `model_catalog_url = "https://your-modeltaps-domain.com/codex/v1/models"`。
- Codex 發送的 `prompt_cache_key` 原樣送到上游，同一個 Codex 會話的請求固定在同一個渠道上。

### 其他編程 Agent

OpenAI 相容的 Agent 指向 `https://your-modeltaps-domain.com/coding-agent/v1`，Anthropic 相容的指向 `https://your-modeltaps-domain.com/coding-agent`。行為與 `/v1`、`/claude` 完全一致，請求會計入編程 Agent 流量。

## 在控制台查看介面目錄

側邊欄「API」分組按模態列出各頁。`/panel/docs` 是總覽，每張模態卡片可進入「介面文檔」或「控制台」。介面文檔頁的內容與本頁一致，但地址已經換成你實際使用的站點地址：

| 模態 | 介面文檔 | 控制台 | 覆蓋的介面 |
| --- | --- | --- | --- |
| 對話 | `/panel/docs/chat` | `/panel/playground/chat` | `/v1/chat/completions`、`/v1/responses`、`/claude/v1/messages`、`/gemini/v1beta/models/{model}:generateContent` |
| 圖像 | `/panel/docs/image` | `/panel/playground/image` | `/v1/images/generations`、`/v1/images/edits`、`/v1/images/variations`，以及 Recraft、Midjourney 擴充介面 |
| 語音 | `/panel/docs/speech` | `/panel/playground/speech` | `/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`，以及 `/v1/realtime` |
| 影片 | `/panel/docs/video` | —（暫未開放） | Kling 的文生影片 / 圖生影片與任務查詢，以及 Gemini Veo 長任務介面 |

每個介面文檔頁都包含「介面端點」「請求示例」「可用模型」與完整文檔連結。示例裏的密鑰是佔位符 `sk-YOUR_TOKEN`，替換成你在「API Key」頁面創建的 API Key 即可運行。

### 控制台

「對話」「圖像」「語音」三頁即控制台，直接調用本站 `/v1`：從示例卡片或底部輸入欄開始，模型與參數以輸入欄上的 chip 選擇。對話為串流輸出、可隨時停止，逐輪顯示首 token 延遲、耗時與用量；圖像可預覽與下載生成結果；語音分「語音合成」「語音識別」兩個分段，可試聽與下載合成音訊，也可上傳或錄製音訊做轉寫。每頁都能查看本次請求的等效程式碼，照著改就能接入自己的程式。

模型選擇器是貼在模型 chip 上的選單，支援搜尋、按能力篩選與按供應商分類，只列出該能力下可用的模型。

- 控制台用的是系統自動為你創建的專用密鑰，不會展示也無需複製，頁面用一個短期憑證發起請求（有效一小時，自動續期），**用量與費用計入你的個人帳戶**，與手動創建的 API Key 分開計費；
- 只在個人上下文可用，切換到組織後頁面會提示並給出出口（組織額度的試用另行支援）；
- 某個能力當前沒有可用模型時，頁面仍可瀏覽，並說明原因與處理入口，不會發出請求；
- 管理員可在系統設定裏把 `builtin_chat_enabled` 關掉，整站停用控制台；
- 舊的 Playground 地址（`/playground`、`/panel/playground`）會自動跳轉到 `/panel/playground/chat`。

#### 對比

側邊欄「API」分組裏的「對比」（`/panel/playground/compare`）把 2–4 個對話模型並排成欄：右上「新增欄」最多加到 4 欄，每欄在欄頭選模型，也可在欄頭覆寫溫度、最大 tokens 與思考深度。底部輸入框的「同步輸入」開著時一條訊息同時發給所有欄，關著只發給當前選中的欄，每欄可單獨停止。「查看程式碼」可在欄間切換，給出該欄合併覆寫參數後的等效請求。

Midjourney 與非同步任務的執行記錄不在目錄頁裏，請到「日誌」頁面的「Midjourney」「非同步任務」標籤查看。

## 關於模型與分組

- 你能調用哪些模型，取決於 API Key 所選的渠道分組，以及 API Key 是否開啟了模型限制。可用清單在「儀表板」的「當前可用模型」中查看，也可以用上面各協議的 models 介面查詢。
- `/claude/v1/messages` 的請求可以呼叫任意對話模型：渠道不支援 Anthropic 協議時，網關會轉換請求。無論呼叫哪個介面，只要渠道支援，Claude 模型都會用 Anthropic 協議發往上游，提示快取因此保持有效。
- Gemini 協議的請求只能路由到 Gemini 類型的渠道。若 Gemini 協議查不到模型，說明當前分組內沒有這類渠道，改用 OpenAI 相容介面即可。
- 請求失敗時可到 [用量與日誌](/hk/guide/usage) 頁面按 API Key 與時間定位具體那一條記錄，明細裏會顯示結束原因與計費構成。

## 相關

- [推理設定](/hk/guide/reasoning)：控制推理模型的思考行為
- [特殊用法](/hk/guide/special)：非通用場景的調用方式
- [API Key 與額度](/hk/guide/tokens)：額度、有效期、模型限制與 IP 白名單
- [常見問題](/hk/guide/faq)：接入報錯排查
