---
title: "Decision Models"
layout: doc
outline: deep
lastUpdated: true
---

# Decision Models

A decision model does not write text. You give it one input and a list of typed questions, and it answers each question with probabilities: yes or no, one of your choices, or a score on your levels. Use it to route support tickets, classify content, check inputs against rules, or grade another model's output.

Modeltaps serves decision models on `POST /v1/decisions`, with the same request and response as the [OpenAI Decisions API](https://developers.openai.com/api/reference/resources/decisions/methods/create). The official OpenAI SDKs work with only a base URL change, whichever vendor's decision model you call.

::: tip Decision models answer on /v1/decisions only
They generate no text, so chat, Responses, Claude and Gemini requests for a decision model are refused with a `400`. The **Models** page marks them with a **Decision** tag.
:::

## Quick start

Replace `https://your-modeltaps-domain.com` with your site address, `sk-replace-with-your-key` with your API key, and `your-decision-model` with a decision model your key can use.

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

The answers come back in question order:

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

## Question types

Every question needs a `type` and `instructions`. `name` is optional and must be unique within a request; an unnamed question comes back with `name: null`.

| Type | Request fields | Answer fields |
| --- | --- | --- |
| `predicate` | `instructions` | `probability`: the probability that the answer is yes |
| `choice` | `instructions`, 1 to 255 `choices`, each with a `value` and an optional `description` | `choice`, `confidence`, `probabilities` |
| `score` | `instructions`, 2 to 10 `levels`, each with a `label` and an optional `description` | `score`, `confidence`, `probabilities` |

- A `score` is the probability-weighted average of the level indexes, which start at `0`.
- `confidence` and `probabilities` appear when the model returns them.
- Choice values can be strings or booleans.
- If the model declines a question, that position holds `{ "type": "refusal", "name": ... }` and the other questions are answered as usual.

## Input

`input` is a string, or an array of user messages. A message's `content` is a string or a list of parts, and every question is asked about the whole input.

| Part | Fields | Notes |
| --- | --- | --- |
| `input_text` | `text` | Plain text. |
| `input_json` | `data` | A JSON object, or a non-empty array of objects. |
| `input_image` | `image_url`, optional `detail` | A base64 data URL (`data:image/png;base64,...`); external URLs are not accepted. `detail` is `low`, `high`, `auto` or `original`. |

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

Image input needs a decision model that reads images; models that read text only answer `400`. The **Models** page shows each model's inputs.

## Billing

A decision request is billed by its input tokens at the model's input price; there is no output charge. The bill uses the input tokens the upstream reports, and a request with refused questions is billed like any other. The **Logs** page shows these requests with the modality **Decisions**.

## Limits and errors

- No streaming: the response comes back in one piece.
- Requests without questions, and requests for a model that is not a decision model, are refused with a `400` before any upstream is called.
- Validation errors use the OpenAI error format:

```json
{
  "error": {
    "message": "model gpt-5-mini is not a decision model; /v1/decisions takes decision models only",
    "type": "invalid_request_error"
  }
}
```

## Finding decision models

Open **Models** in the console and look for the **Decision** tag, or open **API docs → Decisions** for this API with the models your key can use. Decision models come from several vendors, such as Google, OpenAI, Microsoft, HeyGen and Perplexity; which ones you can call depends on your site.
