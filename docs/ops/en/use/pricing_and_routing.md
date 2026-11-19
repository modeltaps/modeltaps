---
title: "Prices, costs and supply order"
layout: doc
outline: deep
lastUpdated: true
---

# Prices, costs and supply order

This page explains how Modeltaps decides what a user pays, what a request costs you, and which supply takes a request. It shows where each setting lives and gives worked examples.

## The three numbers

| Number | What it is | Who sees it |
|---|---|---|
| **Catalog price** | The model's official API price, per million tokens (or per request / minute). | Everyone, on the pricing page |
| **Group ratio** | A multiplier on the catalog price, per user group. The `default` group's ratio is your site's normal price. | Users see the price of their group |
| **Channel cost** | What the upstream charges you: its list price × (1 + its fee rate). | Admins only |

A user always pays **catalog price × group ratio**. That price does not change with the channel that serves the request. Channel costs never reach the user. They decide which supply takes a request, and which supplies the price check holds back.

Your margin on a request is:

> catalog price × group ratio × (1 − payment cost) − channel cost

The **payment cost** is what taking a payment costs you (card fees, currency conversion), as a percentage. Nobody is charged it. It only lowers the revenue the checks compare costs with.

## Where to set what

| What | Where in the console | Setting / field |
|---|---|---|
| Catalog price | **Models** → a model → price; or sync prices from the price sources | model price |
| Your margin | **Users → User groups** → `default` group → ratio (1.12 = 12% over the official price) | `user_groups.ratio` |
| Discounts or premium groups | **Users → User groups** → other groups | `user_groups.ratio` |
| Payment cost | **Platform settings → Billing → Pricing** → Payment cost | `PaymentCostPercent` |
| What-if preview | Same card: type a default group ratio to try (not saved) | — |
| Channel list prices | **Channels** → a channel → Prices (synced from the upstream); relays without a price list: Advanced → Cost ratio | `channel_prices`, `cost_ratio` |
| Channel fee rate | **Channels** → a channel → Advanced → Fee rate, or Top-up records | `fee_rate`, `fee_note` |
| How a tier shares requests | **Platform settings → Gateway → Allocation → Supply mode and cost** → In one tier; per model on its Supplies tab | `SupplyModeDefault`, model supply mode |
| Cost vs. priority | Same card → Cost and priority | `SupplyCostOrder` |
| Tier order | **Platform settings → Gateway → Allocation** → allocation policy; or a model's Supplies tab for manual models | allocation policy, supply priority |

Every setting above, with its type, range and more examples, is in the [settings reference](../settings/index.md). An AI assistant can look them up through the [admin MCP](./admin_mcp.md).

## Channel fee rates

The fee rate is what a provider charges on top of its list price. Most providers charge it when you buy credit, so it is:

> fee rate = cash paid ÷ credit received − 1

New channels start with the provider's published fee. If none is published, they start with the fee seen at checkout:

| Provider | Default | Source |
|---|---|---|
| Requesty | 5% | Top-up fee in its billing docs |
| OpenRouter | 5.5% | Credit purchase fee in its FAQ (minimum $0.80; crypto 5%) |
| Vercel AI Gateway | 3% | Not in Vercel's docs, which only say processing fees are the buyer's. The checkout showed a $3.00 fee on $100 of credit (2026-11-18). |
| Official vendor APIs, other relays | 0% | — |

Record your real top-ups under **Channels → Advanced → Top-up records**: the cash you paid (card and currency fees included) and the credit you received. The channel's fee is then derived from its latest 10 top-ups and replaces the default. Bonus credit never makes the fee negative.

## Which supply takes a request

1. Priorities (from the allocation policy, or a manual model's own numbers) order the supplies.
2. Supplies are split by cost. Only the cheapest take requests. A dearer supply is reached only when every cheaper one is unavailable. Supplies whose cost is unknown come last. Costs within 0.1% count as the same price.
3. Within the supplies that take requests, the supply mode decides: **balance** shares requests by weight, **failover** sends everything to the heaviest.

**Cost and priority** (`SupplyCostOrder`) sets how far step 2 reaches:

- **Priority first** (default): cost only reorders supplies of the same priority. Choose it when a priority means something besides cost, such as quality, speed or a provider you trust.
- **Cost first**: the cheapest supplies of a model take the requests whatever their priority. Priority only orders supplies of the same cost. A cheaper host may be slower or run a quantized model, so check the supplies before you switch.

The model's Supplies tab shows the order requests will try. A backup that waits only because it is dearer is marked **Dearer — backup only**.

## The price check

A supply whose cost is above what the site's normal price brings in loses money on every request it serves. The line is:

> default group ratio × (1 − payment cost)

With a margin set (the line is above 1), new supplies above the line wait for review, and existing backups above it are switched off for review. Supplies already taking requests are left on, and admins are told about them. The **Pricing** card lists every channel's break-even ratio, the supplies that would lose money and what each user group brings in.

Discount groups are not checked: a discount is your own choice. The Pricing card still shows which channels lose money on each group.

## Checking prices against upstream bills

A price list can be wrong in ways no sync notices: it lags a vendor's change, a promotion ended, or a relay adds a markup. The upstream's own bill settles it. Every hour Modeltaps compares the last three days of what upstreams billed with the expected cost it logged from the price list (the channel's list price, without its fee):

- **Per model**, from the cost OpenRouter reports for every request.
- **Per account**, from the daily bill of OpenAI, Anthropic and OpenRouter channels that have an **Upstream management key**. The bill covers the whole account, so use of the same account outside this site makes it look higher.

A difference of 10% or more, over at least 10 requests and $0.10, opens an **Upstream charges** alert on the channel (Platform admin → **Alerts**) and sends one price change notice. The alert links to the channel's **Stats** tab, which shows both amounts per model and per day.

- **Billed more than expected:** requests may lose money. Correct the channel's price, its fee rate or its cost ratio.
- **Billed less than expected:** the cost is set too high, which can hold back supplies at the price check. Lower it the same way.

The alert closes by itself once the costs match again. Gemini and Vertex AI have no bill to compare yet.

## Official price dates

Some vendors announce a price that starts on a date, or an introductory price that ends on one. The built-in official prices carry these dates: on the day, the price changes by itself (unless you edited or locked it), admins get an alert under **Official price dates**, and the price lists are fetched again at once instead of at night.

When an introductory price ends and the vendor has not published the next one, the alert asks you to check the vendor's pricing page, which it links. Set the new price by hand if it changed; the alert stays open until a release carries the new price.

## Examples

### 1. Sell at 12% over the official price

Set the `default` group's ratio to **1.12** and the payment cost to **4%**. Each $1 of catalog price brings in 1.12 × 0.96 = **$1.0752**.

| Channel | Cost per $1 of catalog price | Margin |
|---|---|---|
| Official API | $1.00 | 7.5% |
| Vercel AI Gateway (3%) | $1.03 | 4.4% |
| Requesty (5%) | $1.05 | 2.4% |
| OpenRouter (5.5%) | $1.055 | 1.9% |

A user in the default group calling a model listed at $3 / $15 per million tokens pays $3.36 / $16.80, whichever of these channels serves the request.

### 2. Record a Vercel top-up

You paid **$103.00** for **$100.00** of AI credit. Add a top-up record with paid 103 and credited 100. The channel's fee becomes 3% and its note says it was derived from the top-up. If the next top-up costs $206.40 for $200, the fee becomes (103 + 206.40) ÷ 300 − 1 = 3.13%.

### 3. Two supplies of one priority

`glm-5.2` is supplied by Vercel (cost ×1.03) and Requesty (cost ×1.05) at the same priority, in balance mode. All requests go to Vercel. Requesty takes requests only while Vercel is paused or failing. A default-group user pays the same price either way.

### 4. Cost first across priorities

`gpt-5.1` is supplied by Requesty at the top priority (cost ×1.05) and by the official OpenAI API one priority lower (cost ×1.00).

- **Priority first**: Requesty takes the requests, and OpenAI is the backup.
- **Cost first**: OpenAI takes the requests, and Requesty is the backup, marked dearer.

### 5. A VIP discount that loses money

A `vip` group with ratio **1.05** and a 4% payment cost brings in 1.05 × 0.96 = $1.008 per $1. Requests through the official API still make 0.8%. Through Vercel (×1.03) they lose 2.1%, and through Requesty they lose 4.0%. The Pricing card's group table shows how many channels lose money on `vip`. Either raise the ratio, or accept the loss as the price of the discount.
