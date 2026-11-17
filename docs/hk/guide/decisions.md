---
title: "決策模型"
layout: doc
outline: deep
lastUpdated: true
---

# 決策模型

決策模型不生成文字。你給它一份輸入和一組有類型的問題，它對每個問題給出帶概率的答案：是或否、從你給的選項裏選一個，或按你給的檔位評分。適合工單分流、內容分類、按規則校驗輸入、給其他模型的輸出評分。

Modeltaps 透過 `POST /v1/decisions` 提供決策模型，請求與響應和 [OpenAI Decisions API](https://developers.openai.com/api/reference/resources/decisions/methods/create) 一致。無論調用哪家廠商的決策模型，OpenAI 官方 SDK 只需改接入地址即可使用。

::: tip 決策模型只能用 /v1/decisions 調用
它們不生成文字，所以用決策模型發起的對話、Responses、Claude、Gemini 請求都會被拒絕，返回 `400`。控制台「模型」頁會給它們標上「決策」標籤。
:::

## 快速開始

把 `https://your-modeltaps-domain.com` 換成你的站點地址，`sk-replace-with-your-key` 換成你的 API Key，`your-decision-model` 換成你的 Key 可用的決策模型。

::: code-group

```bash [curl]
curl --request POST \
  --url https://your-modeltaps-domain.com/v1/decisions \
  --header 'Authorization: Bearer sk-replace-with-your-key' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "your-decision-model",
    "input": "The package arrived with a broken screen. I want my money back.",
    "questions": [
      {
        "type": "predicate",
        "name": "damaged",
        "instructions": "Does the customer report a damaged item?"
      },
      {
        "type": "choice",
        "name": "queue",
        "instructions": "Which team should handle this ticket?",
        "choices": [
          { "value": "billing", "description": "Refunds and payments" },
          { "value": "shipping", "description": "Delivery and damaged items" }
        ]
      },
      {
        "type": "score",
        "name": "urgency",
        "instructions": "How urgent is this ticket?",
        "levels": [{ "label": "low" }, { "label": "medium" }, { "label": "high" }]
      }
    ]
  }'
```

```python [Python]
from openai import OpenAI

client = OpenAI(
    api_key="sk-replace-with-your-key",
    base_url="https://your-modeltaps-domain.com/v1",
)

decision = client.decisions.create(
    model="your-decision-model",
    input="The package arrived with a broken screen. I want my money back.",
    questions=[
        {"type": "predicate", "name": "damaged",
         "instructions": "Does the customer report a damaged item?"},
        {"type": "choice", "name": "queue",
         "instructions": "Which team should handle this ticket?",
         "choices": [
             {"value": "billing", "description": "Refunds and payments"},
             {"value": "shipping", "description": "Delivery and damaged items"},
         ]},
        {"type": "score", "name": "urgency",
         "instructions": "How urgent is this ticket?",
         "levels": [{"label": "low"}, {"label": "medium"}, {"label": "high"}]},
    ],
)

for answer in decision.answers:
    print(answer)
```

```ts [TypeScript]
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'sk-replace-with-your-key',
  baseURL: 'https://your-modeltaps-domain.com/v1'
});

const decision = await client.decisions.create({
  model: 'your-decision-model',
  input: 'The package arrived with a broken screen. I want my money back.',
  questions: [
    { type: 'predicate', name: 'damaged', instructions: 'Does the customer report a damaged item?' },
    {
      type: 'choice',
      name: 'queue',
      instructions: 'Which team should handle this ticket?',
      choices: [
        { value: 'billing', description: 'Refunds and payments' },
        { value: 'shipping', description: 'Delivery and damaged items' }
      ]
    },
    {
      type: 'score',
      name: 'urgency',
      instructions: 'How urgent is this ticket?',
      levels: [{ label: 'low' }, { label: 'medium' }, { label: 'high' }]
    }
  ]
});

console.log(decision.answers);
```

:::

答案按問題順序返回：

```json
{
  "model": "your-decision-model",
  "answers": [
    { "type": "predicate", "name": "damaged", "probability": 0.95 },
    {
      "type": "choice",
      "name": "queue",
      "choice": "shipping",
      "confidence": 0.81,
      "probabilities": [
        { "value": "billing", "probability": 0.19 },
        { "value": "shipping", "probability": 0.81 }
      ]
    },
    {
      "type": "score",
      "name": "urgency",
      "score": 1.3,
      "confidence": 0.62,
      "probabilities": [
        { "value": 0, "label": "low", "probability": 0.08 },
        { "value": 1, "label": "medium", "probability": 0.54 },
        { "value": 2, "label": "high", "probability": 0.38 }
      ]
    }
  ],
  "usage": { "input_tokens": 96, "output_tokens": 0, "total_tokens": 96 }
}
```

## 題型

每個問題都要有 `type` 和 `instructions`。`name` 可選，在同一請求內不能重複；沒有名字的問題返回 `name: null`。

| 題型 | 請求欄位 | 返回欄位 |
| --- | --- | --- |
| `predicate` | `instructions` | `probability`：答案為「是」的概率 |
| `choice` | `instructions`，1 至 255 個 `choices`，每項有 `value` 和可選的 `description` | `choice`、`confidence`、`probabilities` |
| `score` | `instructions`，2 至 10 個 `levels`，每項有 `label` 和可選的 `description` | `score`、`confidence`、`probabilities` |

- `score` 是各檔位序號（由 `0` 開始）按概率加權的平均值。
- `confidence` 和 `probabilities` 在模型返回時才出現。
- 選項的值可以是字串或布林值。
- 模型拒絕回答某題時，該位置為 `{ "type": "refusal", "name": ... }`，其他問題照常作答。

## 輸入

`input` 可以是字串，也可以是 user 訊息陣列。訊息的 `content` 是字串或內容塊列表，所有問題都針對整份輸入。

| 內容塊 | 欄位 | 說明 |
| --- | --- | --- |
| `input_text` | `text` | 純文字。 |
| `input_json` | `data` | 一個 JSON 物件，或非空的物件陣列。 |
| `input_image` | `image_url`、可選 `detail` | base64 data URL（`data:image/png;base64,...`），不支援外部連結。`detail` 取 `low`、`high`、`auto` 或 `original`。 |

```json
{
  "model": "your-decision-model",
  "input": [
    {
      "role": "user",
      "content": [
        { "type": "input_text", "text": "Review this order." },
        { "type": "input_json", "data": { "order": { "id": "A-1", "total": 42.5, "status": "refunded" } } }
      ]
    }
  ],
  "questions": [{ "type": "predicate", "name": "refunded", "instructions": "Is the order refunded?" }]
}
```

圖片輸入需要能讀圖的決策模型，只讀文字的模型會返回 `400`。控制台「模型」頁列出了每個模型支援的輸入。

## 計費

決策請求按輸入 token 和模型的輸入價格計費，沒有輸出費用。計費以上游報告的輸入 token 為準；有題目被拒答的請求照常計費。「日誌」頁裏這類請求的模態顯示為「決策」。

## 限制與錯誤

- 不支援串流，響應一次性返回。
- 沒有問題的請求，以及用非決策模型發起的請求，會在調用上游之前被拒絕，返回 `400`。
- 校驗錯誤使用 OpenAI 錯誤格式：

```json
{
  "error": {
    "message": "model gpt-5-mini is not a decision model; /v1/decisions takes decision models only",
    "type": "invalid_request_error"
  }
}
```

## 查找決策模型

在控制台打開「模型」，找帶「決策」標籤的模型；或打開「文件 → 決策」，查看本介面和你的 Key 可用的模型。決策模型來自多家廠商，例如 Google、OpenAI、Microsoft、HeyGen、Perplexity，具體能調用哪些取決於你所在的站點。
