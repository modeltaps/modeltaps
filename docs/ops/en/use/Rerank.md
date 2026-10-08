---
title: "Rerank API"
layout: doc
outline: deep
lastUpdated: true
---

# Rerank API

There is currently no unified standard for rerank APIs, so every provider implements its own. Modeltaps supports three rerank response formats: Jina, Cohere and Siliconflow.

Providers that offer rerank models generally adopt one of these three JSON structures, differing mainly in how usage is reported. Modeltaps only provides rerank parsing for these three channel types.

When adding a rerank model, create a new channel and pick the matching channel type from the three above, based on the provider's documentation and the JSON structure you observe in practice. The custom channel type is really an OpenAI channel and does not include a rerank API.

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
        "text": "Natural organic skincare designed specifically for sensitive skin: experience the gentle care of aloe vera and chamomile extracts. Our products are made for sensitive skin, moisturizing gently and protecting your skin from irritation. Say goodbye to discomfort and hello to healthy, radiant skin."
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
