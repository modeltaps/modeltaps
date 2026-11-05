---
title: "自動路由"
layout: doc
outline: deep
lastUpdated: true
---

# 自動路由

自動路由策略是一個可以像普通模型一樣呼叫的模型名，例如 `auto/coder`。閘道會讀取每個會話的**第一條用戶訊息**，讓一個小的決策模型判斷這是哪類請求，再把會話轉發給管理員為這類請求指定的模型：簡單的工作交給便宜、快的模型，困難的工作交給最強的模型。選定之後整個會話都保持不變。

Claude Code、Codex、Cursor 這類編碼 Agent 無法在請求裏帶路由參數，它們只認一個模型名。策略把路由規則封裝進了這個名字。

## 呼叫策略

把策略名填在 `model` 欄位裏即可。Chat Completions、Claude Messages、Responses 三種介面都支援，含串流與工具呼叫；策略也會像普通模型一樣出現在 `GET /v1/models` 裏。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "auto/coder",
    "messages": [{"role": "user", "content": "給 CLI 加一個 --verbose 參數，並把解析後的參數寫到日誌"}]
  }'
```

響應裏的 `model` 欄位是實際應答的模型（站點開啟「統一返回請求模型名」時除外）。每個被路由的響應還帶一個響應標頭：

```
X-Modeltaps-Route: policy=auto/coder; label=additive; model=claude-haiku-5-5; confidence=0.90; reason=classified
```

| 原因 | 含義 |
| --- | --- |
| `classified` | 決策模型選中了一個標籤，置信度達到策略閾值。 |
| `pinned` | 會話此前已經路由過，這一輪直接沿用，沒有重新決策。 |
| `below_threshold` | 選中了標籤但置信度不夠，由兜底模型應答。 |
| `decision_error` | 決策呼叫失敗、逾時或回答了未知標籤，由兜底模型應答。 |
| `empty_input` | 首條用戶訊息沒有文字（例如只有圖片），由兜底模型應答。 |
| `auxiliary` | 會話還沒路由時先到了 Claude Code 的旁路請求（例如生成會話標題），由兜底模型應答，不做決策。 |

對策略的請求不會因為路由而被拒絕：只要決策做不出來，就由兜底模型應答。

## 會話

決策每個會話只做一次，後續輪次直接複用，所以長對話不會中途換模型，上游的提示快取也能持續命中。會話按以下順序識別：Claude Code 放在 `metadata.user_id` 裏的會話、會話標頭（`session_id`、`session-id`、`x-session-id`、`x-claude-code-session-id`）、OpenAI 的 `prompt_cache_key`、提示前綴（系統提示加首條用戶訊息）。要把一段對話穩定在一個模型上，最可靠的做法是帶一個固定的會話 id；換一個 id 就是一次新決策。只有真正分類出的結果才會固定：`decision_error` 或 `empty_input` 走兜底後，下一輪會重新分類，決策模型短暫故障不會讓新會話一直停在兜底模型上。

會話固定多久由管理員按策略設定（預設一小時）。

## 在編碼 Agent 裏使用

**Claude Code**

```bash
export ANTHROPIC_BASE_URL=https://your-modeltaps-domain.com/claude
export ANTHROPIC_AUTH_TOKEN=sk-proj-1234567890
export ANTHROPIC_MODEL=auto/coder
claude
```

**Codex CLI**：在 `~/.codex/config.toml` 裏指向閘道的 provider 下設定模型：

```toml
model = "auto/coder"
```

**Cursor 及其他 OpenAI 相容客戶端**：把閘道新增為 OpenAI 相容端點，自訂模型名填 `auto/coder`。

編碼 Agent 會往對話裏加自己的上下文：Claude Code 在首條訊息裏放 `<system-reminder>` 區塊，Codex 把 `<environment_context>` 和你的 `AGENTS.md` 指令作為單獨一條訊息放在你的輸入前面。閘道會去掉這些區塊，並跳過只含這些內容的訊息，決策只看你實際輸入的內容。Claude Code 還會用同一個模型發旁路請求，例如生成會話標題；這類請求不帶工具，不參與分類：會話路由之前走兜底模型，之後跟隨會話。

## 計費與日誌

- 被路由的請求按實際應答模型的價格計費，策略本身沒有價格。模型目錄裏策略顯示「按目標模型計費」而不是自己的價格。
- 決策呼叫每個會話向你計費一次，按決策模型的價格，單獨記一條標為決策呼叫的日誌。站點管理員可以關閉這項計費。
- 日誌裏被路由的請求會顯示策略、選中的標籤、置信度和原因；日誌可以按路由策略篩選。

## 設定策略（管理員）

策略在管理後台的 **閘道 → 自動路由** 裏建立，由以下部分組成：

- **模型名**：客戶端呼叫的名字，如 `auto/coder`。只能用字母、數字和 `. _ / -`，不能與已有模型重名。建立後出現在 `/v1/models` 裏，並能像普通模型一樣加進令牌和組織白名單。白名單按策略名判定，目標模型不必在呼叫方的白名單裏，因此可以讓昂貴或隱藏的模型只經策略可達。
- **決策模型**：給首條訊息分類的小而快的對話模型。閘道提供的任何對話模型都可以，選一個聽指令的。它本身不能是策略。
- **問題與路由**：一句話的問題，加上 2 到 16 個標籤，每個標籤帶簡短描述和目標模型。描述請求本身而不是模型，標籤要互不重疊：重疊會分散概率、把請求推向兜底。三個範本（編碼改動大小、客服分流、推理深度）可作起點。
- **兜底模型與閾值**：決策不可信時由兜底應答，放最穩的模型。閾值（0 到 1，0.5 是不錯的起點）和決策模型自報的置信度比較，調高會讓更多模稜兩可的請求走兜底。置信度是自報的，當排序看，別當概率。
- **進階**：會話固定多久（0 表示每次都分類）、送給決策模型的首條訊息截斷長度、決策逾時。

**試一試** 標籤頁只跑決策這一步：貼上一條首條訊息，可並列比較多個決策模型，不轉發、不計費、不寫會話固定記錄。啟用策略前，用日誌裏的真實首條訊息測一遍。

決策計費開關在 **系統設定 → 閘道行為 → 自動路由**。
