---
title: "Rerank API"
layout: doc
outline: deep
lastUpdated: true
---

# Rerank API

rerank の API には現時点で統一された標準がなく、各プロバイダーがそれぞれ独自の実装を採用しています。Modeltaps は現在、Jina・Cohere・Siliconflow の 3 種類の rerank レスポンス形式に対応しています。

rerank モデルを提供するプロバイダーは通常、この 3 つのいずれかの JSON 構造をレスポンス形式として採用しており、違いは主に Usage の返し方にあります。システムは以下の 3 種類のプロバイダーチャネルタイプに対してのみ rerank の解析に対応しています。

Rerank モデルを追加する際は、新しいチャネルを作成し、プロバイダーのドキュメントと実際に確認した JSON 構造に基づいて、上記 3 種類から該当するチャネルタイプを選択してください。カスタムチャネルは実質的に OpenAI タイプであり、その下には rerank の API は含まれません。

## Jina

```json
{
  "model": "jina-reranker-v2-base-multilingual",
  "usage": {
    "total_tokens": 815
  },
  "results": [
    {
      "index": 0,
      "document": {
        "text": "Organic skincare for sensitive skin with aloe vera and chamomile: Imagine the soothing embrace of nature with our organic skincare range, crafted specifically for sensitive skin. Infused with the calming properties of aloe vera and chamomile, each product provides gentle nourishment and protection. Say goodbye to irritation and hello to a glowing, healthy complexion."
      },
      "relevance_score": 0.8783142566680908
    },
    {
      "index": 6,
      "document": {
        "text": "敏感肌のために設計されたオーガニックスキンケア製品：アロエベラとカモミールのエキスによる自然のいたわりを体験してください。当社のスキンケア製品は敏感肌のために特別に設計され、やさしく潤いを与えながら、刺激から肌を守ります。肌の不快感にお別れを告げ、健やかな輝きを手に入れましょう。"
      },
      "relevance_score": 0.8783142566680908
    },
    {
      "index": 4,
      "document": {
        "text": "Cuidado de la piel orgánico para piel sensible con aloe vera y manzanilla: Descubre el poder de la naturaleza con nuestra línea de cuidado de la piel orgánico, diseñada especialmente para pieles sensibles. Enriquecidos con aloe vera y manzanilla, estos productos ofrecen una hidratación y protección suave. Despídete de las irritaciones y saluda a una piel radiante y saludable."
      },
      "relevance_score": 0.8624675869941711
    }
  ]
}
```

## Cohere

```json
{
  "id": "e6b994e7-1158-4216-bdec-3ea4246434d0",
  "results": [
    {
      "index": 0,
      "relevance_score": 0.9197076
    },
    {
      "index": 4,
      "relevance_score": 0.9008183
    },
    {
      "index": 8,
      "relevance_score": 0.8800674
    }
  ],
  "meta": {
    "api_version": {
      "version": "2"
    },
    "billed_units": {
      "search_units": 1
    }
  }
}
```

## Siliconflow

```json
{
  "id": "0195c4bc8788725da3006f0779171780",
  "results": [
    { "index": 8, "relevance_score": 0.9983834 },
    { "index": 0, "relevance_score": 0.9983059 },
    { "index": 6, "relevance_score": 0.9961606 }
  ],
  "meta": {
    "billed_units": {
      "input_tokens": 896,
      "output_tokens": 0,
      "search_units": 0,
      "classifications": 0
    },
    "tokens": { "input_tokens": 896, "output_tokens": 0 }
  }
}
```
