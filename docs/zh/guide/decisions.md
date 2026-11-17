---
title: "决策模型"
layout: doc
outline: deep
lastUpdated: true
---

# 决策模型

决策模型不生成文本。你给它一份输入和一组带类型的问题，它对每个问题给出带概率的答案：是或否、从你给的选项里选一个，或按你给的档位打分。适合工单分流、内容分类、按规则校验输入、给其他模型的输出打分。

Modeltaps 通过 `POST /v1/decisions` 提供决策模型，请求与响应和 [OpenAI Decisions API](https://developers.openai.com/api/reference/resources/decisions/methods/create) 一致。无论调用哪家厂商的决策模型，OpenAI 官方 SDK 只需改接入地址即可使用。

::: tip 决策模型只能用 /v1/decisions 调用
它们不生成文本，所以用决策模型发起的对话、Responses、Claude、Gemini 请求都会被拒绝，返回 `400`。控制台「模型」页会给它们标上「决策」标签。
:::

## 快速开始

把 `https://your-modeltaps-domain.com` 换成你的站点地址，`sk-replace-with-your-key` 换成你的 API Key，`your-decision-model` 换成你的 Key 可用的决策模型。

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

答案按问题顺序返回：

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

## 题型

每个问题都要有 `type` 和 `instructions`。`name` 可选，在同一请求内不能重复；没有名字的问题返回 `name: null`。

| 题型 | 请求字段 | 返回字段 |
| --- | --- | --- |
| `predicate` | `instructions` | `probability`：答案为「是」的概率 |
| `choice` | `instructions`，1 到 255 个 `choices`，每项有 `value` 和可选的 `description` | `choice`、`confidence`、`probabilities` |
| `score` | `instructions`，2 到 10 个 `levels`，每项有 `label` 和可选的 `description` | `score`、`confidence`、`probabilities` |

- `score` 是各档位序号（从 `0` 开始）按概率加权的平均值。
- `confidence` 和 `probabilities` 在模型返回时才出现。
- 选项的值可以是字符串或布尔值。
- 模型拒绝回答某题时，该位置为 `{ "type": "refusal", "name": ... }`，其他问题照常作答。

## 输入

`input` 可以是字符串，也可以是 user 消息数组。消息的 `content` 是字符串或内容块列表，所有问题都针对整份输入。

| 内容块 | 字段 | 说明 |
| --- | --- | --- |
| `input_text` | `text` | 纯文本。 |
| `input_json` | `data` | 一个 JSON 对象，或非空的对象数组。 |
| `input_image` | `image_url`、可选 `detail` | base64 data URL（`data:image/png;base64,...`），不支持外部链接。`detail` 取 `low`、`high`、`auto` 或 `original`。 |

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

图片输入需要能读图的决策模型，只读文本的模型会返回 `400`。控制台「模型」页列出了每个模型支持的输入。

## 计费

决策请求按输入 token 和模型的输入价格计费，没有输出费用。计费以上游报告的输入 token 为准；有题目被拒答的请求照常计费。「日志」页里这类请求的模态显示为「决策」。

## 限制与错误

- 不支持流式，响应一次性返回。
- 没有问题的请求、以及用非决策模型发起的请求，会在调用上游之前被拒绝，返回 `400`。
- 校验错误使用 OpenAI 错误格式：

```json
{
  "error": {
    "message": "model gpt-5-mini is not a decision model; /v1/decisions takes decision models only",
    "type": "invalid_request_error"
  }
}
```

## 查找决策模型

在控制台打开「模型」，找带「决策」标签的模型；或打开「接口文档 → 决策」，查看本接口和你的 Key 可用的模型。决策模型来自多家厂商，例如 Google、OpenAI、Microsoft、HeyGen、Perplexity，具体能调用哪些取决于你所在的站点。
