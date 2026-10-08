---
title: "Price Updates"
layout: doc
outline: deep
lastUpdated: true
---

# Price Updates

Modeltaps keeps all model prices in a single price table maintained by administrators on the `Operations --> Model Prices` page. On first startup the table is initialized from the built-in default prices; from then on you can maintain it manually or sync it in bulk from a price service.

## Manual maintenance

On the `Operations --> Model Prices` page you can add, edit, bulk-modify and delete model prices. Each entry contains the following fields:

- **Model name**: must match the `model` field in the request. Prefix matching with a trailing `*` is supported (for example `gpt-4o*`).
- **Billing type**: `tokens` bills per token, `times` bills per request.
- **Model owner**: the `channel_type`, which determines the provider a model is grouped under on the "Available Models" page. The owner list is maintained on `Operations --> Model Owners`; the owner ID in a price entry must match an ID in that list, otherwise the page cannot group models by provider correctly.
- **Input / output ratio**: the ratio relative to the base quota. For per-request billing, set both to the same value.
- **Locked**: when checked, the "overwrite" bulk sync mode leaves this entry untouched — useful for protecting manually verified models.
- **Extra ratios**: ratios for special tokens such as cache reads/writes and audio.
- **Long-context tier**: once input tokens exceed the threshold, the whole request uses an alternative pair of input/output ratios.

## Bulk sync from a price service

Click `Update Prices` on the page and enter a URL that returns a price table in JSON. After fetching, Modeltaps shows a comparison of "new models" and "price changes"; once you have reviewed it, choose a sync mode:

- **Add only**: writes only models that do not yet exist; existing prices are untouched.
- **Overwrite**: deletes and recreates all unlocked prices so they exactly match the price table.
- **Update only**: updates existing models only; nothing new is added.

::: tip Recommendation
In production, start with "add only" and review changed entries one by one before applying them, so a single overwrite does not disturb ratios you have already tuned.
:::

The same dialog also has a button that pulls real prices for direct channels (OpenAI / Anthropic / Gemini and others) from the model catalog and converts them into ratios. Those results go through the same comparison preview and never overwrite locked prices.

### Price table JSON format

The price service must return either an array of price objects or an object with a `data` field (both are accepted):

```json
[
  {
    "model": "gpt-4o",
    "type": "tokens",
    "channel_type": 1,
    "input": 1.25,
    "output": 5,
    "locked": false,
    "extra_ratios": {
      "cached_read": 0.1,
      "cached_write": 1.25
    },
    "long_context": {
      "threshold": 272000,
      "input_ratio": 2,
      "output_ratio": 1.5
    }
  },
  {
    "model": "mj_imagine",
    "type": "times",
    "channel_type": 34,
    "input": 50,
    "output": 50
  }
]
```

Wrapped in `data`:

```json
{
  "data": [{ "model": "gpt-4o", "type": "tokens", "channel_type": 1, "input": 1.25, "output": 5 }]
}
```

Field reference:

| Field | Type | Description |
| --- | --- | --- |
| `model` | string | Model name. Required; a trailing `*` enables prefix matching |
| `type` | string | Billing type, `tokens` or `times`. Required |
| `channel_type` | int | Model owner ID; must match an ID under `Operations --> Model Owners` |
| `input` | float | Input ratio; for per-request billing this is the per-call ratio |
| `output` | float | Output ratio; for per-request billing set it to the same value as `input` |
| `locked` | bool | Whether the entry is locked; locked prices are not modified by overwrite mode |
| `extra_ratios` | object | Extra ratios, keyed by ratio name (such as `cached_read`, `cached_write`) with the ratio as the value |
| `long_context` | object | Long-context tier containing `threshold`, `input_ratio` and `output_ratio`; a `threshold` of 0 means disabled |

## Pointing to a self-hosted price service

To use a price table you maintain yourself, set the default URL through the `UPDATE_PRICE_SERVICE` environment variable; the update dialog on the price page is pre-filled with it:

```bash
UPDATE_PRICE_SERVICE=https://prices.your-domain.com/prices.json
```

Combined with the variables below, the program can also sync periodically after startup:

- `AUTO_PRICE_UPDATES`: whether automatic updates are enabled.
- `AUTO_PRICE_UPDATES_MODE`: automatic update mode — `add` / `overwrite` / `update` / `system`. Defaults to `system` (initialize from the built-in price table only, never contacting a price service).
- `AUTO_PRICE_UPDATES_INTERVAL`: automatic update interval, in minutes.

See [Environment Variables](../deployment/env) for full descriptions.

::: warning Note
Automatic updates rewrite the price table without human confirmation. If your ratios are manually verified, keep `AUTO_PRICE_UPDATES_MODE=system`, mark important models as locked, and only sync after fetching manually on the price page and reviewing each entry.
:::
