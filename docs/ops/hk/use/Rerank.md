---
title: "Rerank 介面"
layout: doc
outline: deep
lastUpdated: true
---

# Rerank 介面

由於 rerank 介面目前沒有統一標準，各供應商均有各自的實現方案。Modeltaps 目前支援下述三種 rerank 返回格式，分別為 Jina、Cohere、Siliconflow。

通常提供 rerank 模型的供應商會選擇三者之一的 JSON 結構作為其返回形式，差別主要在於 Usage 的返回方案。系統僅對以下三種供應商渠道類型提供 rerank 的解析方案。

新增 Rerank 模型時，請建立一個新的渠道，並按照供應商文檔以及實測得到的 JSON 結構，從上述三種中確定對應的渠道類型。自訂渠道實為 OpenAI 類型，其下不包含 rerank 介面。

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
        "text": "針對敏感肌專門設計的天然有機護膚產品：體驗由蘆薈和洋甘菊提取物帶來的自然呵護。我們的護膚產品特別為敏感肌設計，溫和滋潤，保護您的肌膚不受刺激。讓您的肌膚告別不適，迎來健康光彩。"
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
