---
title: "Billing and Pricing"
layout: doc
outline: deep
lastUpdated: true
---

# Billing and Pricing

This page explains how Modeltaps calculates the cost of each request and where you can see prices and billing details.

## Quota and money

Your balance is recorded in **quota**, which the site converts to money at a fixed rate for display: **500,000 quota = $1**.

Amounts in the interface are usually shown directly in US dollars; if currency display is disabled for the site, the raw quota numbers are shown instead. Unit prices on the pricing page can be switched between **$ / 1K tokens** and **$ / 1M tokens**.

## Where to find prices

Open the **Available models** page to see the input and output price of every model:

- **Group**: switch groups in the top right. Prices follow the group ratio, which is marked with badges such as `x1` or `x0.5`; models with a ratio of 0 show as **Free**.
- **Show available only**: enabled by default, listing only models your current group can call.
- **Type**: distinguishes **usage-based** from **per-request** pricing.
- **Unit**: switch between K and M units in the **Display** menu.
- **View details**: click the eye icon at the end of a row to see the model's price in each group along with extra ratios for cache, audio, reasoning and so on.

If a model shows **Price not configured**, the site has not set a price for it yet and calls may be rejected outright.

## Usage-based pricing (tokens)

Most chat and text models are billed per token, with input and output priced separately:

> Cost = input tokens × input unit price + output tokens × output unit price

The unit price is the price shown on the pricing page, that is **model unit price × group ratio**. Input and output each use their own price, with no further multipliers applied.

## Per-request pricing (times)

Image, music and similar models are billed per call: every successful call is charged a fixed per-call price regardless of how many tokens were consumed. Such models show as **Per request** in the **Type** column of the pricing page.

## Extra ratios

Some usage falls into special token types, such as cached input, audio, or the reasoning process. Each is converted using its own ratio and counted on the input or output side:

| Name | Counted on | Meaning |
| --- | --- | --- |
| `cached_tokens` | Input | Input tokens that hit the prompt cache |
| `cached_write_tokens` | Input | Cache write (5-minute TTL) |
| `cached_write_1h_tokens` | Input | Cache write (1-hour TTL) |
| `cached_read_tokens` | Input | Cache read |
| `openai_cache_write_tokens` | Input | OpenAI cache write |
| `input_audio_tokens` | Input | Audio input |
| `input_text_tokens` | Input | Text input |
| `input_image_tokens` | Input | Image input |
| `output_audio_tokens` | Output | Audio output |
| `output_image_tokens` | Output | Image output |
| `output_text_tokens` | Output | Text output |
| `reasoning_tokens` | Output | Tokens consumed by the reasoning process |

Ratios are straightforward: **a ratio of 1 means the same price as a normal token, above 1 is more expensive, below 1 is cheaper**. For example, cache reads default to a ratio of 0.1, so cached input is billed at one tenth; cache writes with a 1-hour TTL default to a ratio of 2.

The actual ratios for a model are whatever **View details** shows; types without a specific configuration are billed as normal tokens.

## Long-context tiers

A few models have long-context tiers configured: when the input tokens of a request exceed the configured threshold, the input and output prices for the entire request are each multiplied by the corresponding tier ratio. Below the threshold, prices are unchanged.

## Pre-deduction and settlement

When a request starts, the system pre-deducts some quota based on the input tokens; after the request finishes it settles against actual usage, refunding or charging the difference. As a result:

- if the balance or API key quota is insufficient, the request is rejected before it is sent;
- if quota runs out during a streaming request, the connection is interrupted;
- the cost shown in the logs is the final settled amount.

## Reviewing billing details

- **Usage** page: spend, token volume and request counts by time, model, API key and application.
- **Logs** page: the cost of each individual request; expand a record to see the actual input price, output price, group ratio, raw charge and final cost for that call, along with the breakdown of cache, reasoning and other token types.
- **Transactions** tab of the **Billing** page (opened from the user menu at the bottom of the sidebar): top-ups, redemptions, system adjustments and other accounting records, which do not appear in the request logs.

Redemption codes may be restricted to one kind of account: a **Personal only** code can only be redeemed into a personal account, and an **Organization only** code can only be redeemed into an organization quota pool from the organization context. Redeeming in the wrong context is rejected and the code stays usable — see [Organizations](/guide/organization).

## Related pages

- [API keys and quota](/guide/tokens): API key quota, periodic resets and model restrictions
- [Usage and logs](/guide/usage): spend and request details
- [FAQ](/guide/faq): troubleshooting quota and errors
