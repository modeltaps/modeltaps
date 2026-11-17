---
title: "意思決定モデル"
layout: doc
outline: deep
lastUpdated: true
---

# 意思決定モデル

意思決定モデルはテキストを生成しません。1 つの入力と型付きの質問の一覧を渡すと、各質問に確率付きで答えます：はい/いいえ、指定した選択肢から 1 つ、または指定した段階での採点です。サポートチケットの振り分け、コンテンツの分類、ルールに沿った入力チェック、他のモデルの出力の採点に使えます。

Modeltaps は `POST /v1/decisions` で意思決定モデルを提供し、リクエストとレスポンスは [OpenAI Decisions API](https://developers.openai.com/api/reference/resources/decisions/methods/create) と同じです。どのベンダーの意思決定モデルを呼び出す場合も、OpenAI 公式 SDK はベース URL を変えるだけで使えます。

::: tip 意思決定モデルは /v1/decisions でのみ呼び出せます
テキストを生成しないため、意思決定モデルを指定したチャット、Responses、Claude、Gemini のリクエストは `400` で拒否されます。コンソールの「モデル」ページでは「意思決定」タグが付きます。
:::

## クイックスタート

`https://your-modeltaps-domain.com` をサイトのアドレスに、`sk-replace-with-your-key` を API キーに、`your-decision-model` をそのキーで使える意思決定モデルに置き換えてください。

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

回答は質問の順に返ります：

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

## 質問の種類

各質問には `type` と `instructions` が必要です。`name` は任意で、同じリクエスト内で重複できません。名前のない質問は `name: null` で返ります。

| 種類 | リクエストのフィールド | 回答のフィールド |
| --- | --- | --- |
| `predicate` | `instructions` | `probability`：答えが「はい」である確率 |
| `choice` | `instructions`、1〜255 個の `choices`（それぞれ `value` と任意の `description`） | `choice`、`confidence`、`probabilities` |
| `score` | `instructions`、2〜10 個の `levels`（それぞれ `label` と任意の `description`） | `score`、`confidence`、`probabilities` |

- `score` は各段階の番号（`0` から）を確率で重み付けした平均です。
- `confidence` と `probabilities` はモデルが返した場合にのみ含まれます。
- 選択肢の値は文字列または真偽値です。
- モデルが質問への回答を拒否した場合、その位置は `{ "type": "refusal", "name": ... }` になり、他の質問は通常どおり回答されます。

## 入力

`input` は文字列、または user メッセージの配列です。メッセージの `content` は文字列またはパーツのリストで、すべての質問は入力全体について問われます。

| パーツ | フィールド | 説明 |
| --- | --- | --- |
| `input_text` | `text` | プレーンテキスト。 |
| `input_json` | `data` | JSON オブジェクト、または空でないオブジェクトの配列。 |
| `input_image` | `image_url`、任意の `detail` | base64 の data URL（`data:image/png;base64,...`）。外部 URL は使えません。`detail` は `low`、`high`、`auto`、`original` のいずれか。 |

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

画像入力には画像を読める意思決定モデルが必要です。テキストのみのモデルは `400` を返します。各モデルの入力はコンソールの「モデル」ページで確認できます。

## 課金

意思決定リクエストは入力トークンとモデルの入力単価で課金され、出力の料金はかかりません。課金はアップストリームが報告した入力トークンに基づき、回答を拒否された質問を含むリクエストも通常どおり課金されます。「ログ」ページでは、これらのリクエストのモダリティが「意思決定」と表示されます。

## 制限とエラー

- ストリーミングには対応していません。レスポンスは一度に返ります。
- 質問のないリクエストや、意思決定モデル以外を指定したリクエストは、アップストリームを呼び出す前に `400` で拒否されます。
- 検証エラーは OpenAI のエラー形式です：

```json
{
  "error": {
    "message": "model gpt-5-mini is not a decision model; /v1/decisions takes decision models only",
    "type": "invalid_request_error"
  }
}
```

## 意思決定モデルを探す

コンソールの「モデル」を開いて「意思決定」タグの付いたモデルを探すか、「ドキュメント → 意思決定」を開くと、この API とキーで使えるモデルを確認できます。意思決定モデルは Google、OpenAI、Microsoft、HeyGen、Perplexity など複数のベンダーが提供しており、呼び出せるモデルはサイトによって異なります。
