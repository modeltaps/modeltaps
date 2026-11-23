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
| Call order | **Providers** page: drag channels into tiers; channels of one tier share it by percentage. A single model can keep its own order on its Supplies tab | allocation policy, supply priority and weight |
| Order by | Top of the **Providers** page, or **Platform settings → Supply allocation → Supply routing and order** | `SupplyCostOrder` |

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

1. The call order puts the supplies in tiers (from the providers page, or the own numbers of a model set by hand).
2. With order by **cheaper first**, cost orders them too (see below).
3. Supplies of one tier share requests by weight. To keep a supply as a backup only, put it in the tier below.

**Order by** (`SupplyCostOrder`) decides whether cost takes part:

- **Call order** (default): cost does not order supplies. Supplies of one tier share requests by weight; a cheaper one does not take them all. Choose it when the order means something besides cost, such as quality, speed or a provider you trust.
- **Cheaper first**: the cheapest supplies of a model take the requests whatever the call order. A dearer supply is reached only when every cheaper one is unavailable, and supplies whose cost is unknown come last. Two guards keep a cheap but unreliable supply from taking the traffic on price alone:
  - **Minimum saving** (`SupplyCostMinSavingPercent`, default 5%): a supply must be this much cheaper than another to go ahead of it. Closer costs count as the same and share by priority and weight.
  - **Minimum success rate** (`SupplyCostMinSuccessPercent`, default 95%): a supply below this success rate over the current and the previous hour is ordered as if its cost were unknown, behind the supplies that serve well, until it recovers. A supply with fewer than 20 attempts is not judged.

  A cheaper host may also be slower or run a quantized model, so check the supplies before you switch.

The model's Supplies tab shows the order requests will try. A backup that waits only because it is dearer is marked **Dearer — backup only**; a cheaper supply held back by its success rate is marked **Low success rate**.

### Hosts inside a gateway (Vercel AI Gateway)

On Vercel AI Gateway many open models are hosted by several providers at different prices and quality. Left alone, Vercel picks one per request by its own priority and recent health and bills that provider's price, so neither the cost nor what answers is known in advance. For such a model the price sync reads every host and orders them, and every request carries that order (`providerOptions.gateway.order`):

1. The vendor's own hosting first (Alibaba for Qwen, Z.ai for GLM).
2. Other hosts must qualify: serving, up at least the **minimum success rate** over the last day, full precision, a context no shorter than the model's, prompt caching when the vendor caches, and a price no lower than 30% of the vendor's. Qualifying hosts follow, cheapest first.
3. With **cheaper first**, qualifying hosts cheaper than the vendor by the **minimum saving** go ahead of it. A vendor that is down or up too little goes after the qualifying hosts.

When a host up the order is unavailable, Vercel still moves to the next, so requests do not fail for it. The channel's cost is the first host's price, and the price details list the order. A model whose vendor Vercel does not list is left to Vercel's routing at its listed price. The order is refreshed with the prices (at startup, daily, and when the channel is saved).

Vercel tells us with every request which host served it and what it charged: the host is kept in the log details as **Served by host**, and the charge as the upstream cost. The cost and margin figures use that charge (with the channel's fee) instead of an estimate from the price list, so the cost stays right when Vercel falls back to another host.

## When a supply fails

What happens depends on what the upstream said, not on the failure alone:

| What happened | How it is recognised | This request | The supply afterwards |
|---|---|---|---|
| The request itself is refused | A 400 about the request: a malformed body, an unsupported parameter, a context that is too long, refused content | Returned to the caller, not tried elsewhere | Nothing, and it does not lower the supply's success rate |
| Out of balance or quota | 402, `insufficient_quota`, the known "balance too low" messages, or a balance refresh that finds nothing left | Moves to the next supply | The whole channel is paused for 30 minutes and tested before it resumes; admins are told. A balance refresh that finds money ends the pause |
| Key or account rejected | 401, `invalid_api_key`, `account_deactivated`, a disable keyword | Moves on | The whole channel is paused for an hour and tested before it resumes |
| The channel does not have the model | "model not found" and the like | Moves on | That model is paused on the channel |
| Rate limited or over capacity, with a time to come back | 429, 503 or 529 with `Retry-After` (or the provider's own reset time) | Moves on | That model is paused on the channel until that time |
| Rate limited, no time given | 429 | Moves on | That model cools down for the rate limit cooldown (5 seconds by default) |
| Upstream error | 500, 502, 503, 504, a dropped connection | Moves on | Paused only on the third in a minute, for 2 minutes, and tested before it resumes |
| No answer in time | The upstream accepted the request and stayed silent | Moves on while the retry time limit allows | A test request is sent at once; the model is paused only when that fails too |
| The answer had already started | The stream broke after its first bytes | Cannot move: the caller has part of the answer | Counted like an upstream error |

Two things follow from keeping "this request failed" apart from "this supply is down":

- **A request is saved on little evidence, a supply is judged on more.** One failure is enough to try the next supply. Pausing a supply takes an upstream that says so (balance, key, a time to come back), a failure that repeats, or a test request that fails as well. A request that got no answer in time proves nothing by itself, since it may have been a long request on a healthy channel: it is never counted, only checked.
- **A session stays where its prompt cache is.** With session affinity on, a session's requests go to the channel that served it. When that channel answers one of them with an upstream error, the request is sent to it once more before it moves, because such errors usually pass and another channel has to write the cache again. This extra try is skipped when retries are set to 0. A session that did move stays on its new channel, also after the old one is back; new sessions start on the first supply again.

The table shows the built-in pause rules. **Gateway settings → Resilience** replaces them with your own.

## The price check

A supply whose cost is above what the site's normal price brings in loses money on every request it serves. The line is:

> default group ratio × (1 − payment cost)

The check compares the input and output cost with the catalog price. Cache reads are not part of it: they are a share of the input, and a channel that charges more for them often charges much less for the rest, so its requests still earn. A supply's price details show what cache reads cost on the channel next to what you sell them at, and say when they cost more, so you can judge a supply whose requests are mostly cache hits.

With a margin set (the line is above 1), new supplies above the line wait for review, and existing backups above it are switched off for review. Supplies already taking requests are left on, and admins are told about them. The **Pricing** card lists every channel's break-even ratio, the supplies that would lose money and what each user group brings in.

Discount groups are not checked: a discount is your own choice. The Pricing card still shows which channels lose money on each group.

## Checking prices against upstream bills

A price list can be wrong in ways no sync notices: it lags a vendor's change, a promotion ended, or a relay adds a markup. The upstream's own bill settles it. Every hour Modeltaps compares the last three days of what upstreams billed with the expected cost it logged from the price list (the channel's list price, without its fee):

- **Per model**, from the cost OpenRouter, Vercel AI Gateway and Cheaper Inference report for every request.
- **Per account**, from the daily bill of OpenAI, Anthropic and OpenRouter channels that have an **Upstream management key**. The bill covers the whole account, so use of the same account outside this site makes it look higher.

A difference of 10% or more, over at least 10 requests and $0.10, opens an **Upstream charges** alert on the channel (Platform admin → **Alerts**) and sends one price change notice. The alert links to the channel's **Stats** tab, which shows both amounts per model and per day.

- **Billed more than expected:** requests may lose money. Correct the channel's price, its fee rate or its cost ratio.
- **Billed less than expected:** the cost is set too high, which can hold back supplies at the price check. Lower it the same way.

The alert closes by itself once the costs match again. Gemini and Vertex AI have no bill to compare yet.

For a single request, the log details show admins its channel cost, where the figure comes from (the upstream's own charge, the channel's price list, its cost ratio or the official price) and the channel fee in it. A charge the upstream reports after the request replaces the estimate. A request whose cost is not known says so, and the profit per minute on the statistics page leaves it out.

## Official price dates

Some vendors announce a price that starts on a date, or an introductory price that ends on one. The built-in official prices carry these dates: on the day, the price changes by itself (unless you edited or locked it), admins get an alert under **Official price dates**, and the price lists are fetched again at once instead of at night.

When an introductory price ends and the vendor has not published the next one, the alert asks you to check the vendor's pricing page, which it links. Set the new price by hand if it changed; the alert stays open until a release carries the new price.

## Off-peak prices

Some vendors charge less off peak. DeepSeek charges half price outside 01:00–04:00 and 06:00–10:00 UTC, Monday to Friday; the built-in official prices carry those hours, so users are charged half price off peak too. A request is priced by the time it started, and its cost and the upstream charge check follow the same rule; the log details show "Off-peak price".

The model page and the price list still show the peak price. DeepSeek also counts Chinese public holidays as off peak; Modeltaps does not know them yet and charges the full price on those days, so it never charges less than the vendor. **Charge full price** on a model's price form removes its off-peak discount, and a locked price never gets one added. Through another channel (such as OpenRouter) the cost comes from that channel's own price list, without a discount.

## When a price source has problems

When a channel's price list or models.dev cannot be read, Modeltaps keeps the prices it read last and shows when the reads started failing, and why, on the channel's **Models** tab. After two failed reads over at least 20 hours, a **Price source issue** alert opens and one notice is sent; the next good read closes it. Only sources an automatic sync reads are watched.

A new list that lost more than half of its models, or where the share of models without a price grew by 20 points, usually means the platform changed its format. It is not stored, and the stored prices stay in use. If the platform really dropped that many models, click **Use the new list** on the channel's **Models** tab.

After each daily models.dev sync, the built-in official prices are compared with the vendor's own entry there. Models whose input or output price differs by 10% or more are listed in a **Built-in official prices** alert. It changes nothing: models.dev can lag or be wrong too, so check the vendor's pricing page. DeepSeek's models.dev entries show the off-peak price (half the list price); that counts as a match.

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

`glm-5.2` is supplied by Vercel (cost ×1.03) and Requesty (cost ×1.05) in one tier, with weights 3 and 1.

- **Call order**: Vercel takes about 75% of the requests and Requesty about 25%.
- **Cheaper first**: they are 2% apart, within the 5% minimum saving, so they count as the same cost and still share 75 / 25. With the minimum saving at 0, every request goes to Vercel, and Requesty takes requests only while Vercel is paused or failing.

A default-group user pays the same price either way.

### 4. Cheaper first across tiers

`gpt-5.1` is supplied by Requesty in tier 1 (cost ×1.05) and by a discount relay in tier 2 (cost ×0.80).

- **Call order**: Requesty takes the requests, and the relay is the backup.
- **Cheaper first**: the relay is 24% cheaper and takes the requests; Requesty is the backup, marked dearer. When the relay fails 10% of its requests over the last two hours, it is below the 95% success rate and Requesty takes the requests back until the relay recovers.

### 5. A VIP discount that loses money

A `vip` group with ratio **1.05** and a 4% payment cost brings in 1.05 × 0.96 = $1.008 per $1. Requests through the official API still make 0.8%. Through Vercel (×1.03) they lose 2.1%, and through Requesty they lose 4.0%. The Pricing card's group table shows how many channels lose money on `vip`. Either raise the ratio, or accept the loss as the price of the discount.
