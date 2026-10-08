---
title: "Usage and Logs"
layout: doc
outline: deep
lastUpdated: true
---

# Usage and Logs

Modeltaps offers three levels of usage visibility: the **Dashboard** for the current snapshot, **Usage** for trends and attribution, and **Logs** for tracing every individual request.

## Dashboard

The default page after signing in. It shows today's request count, today's spend, today's token volume and available quota, plus seven-day trends, seven-day model usage, cache hit rate, discount savings and other summary information, and lists the models currently available to you.

## Usage

The **Usage** page shows how spend, token volume and request counts change over time.

### Time range

The time picker at the top supports relative ranges (last 15 minutes / 30 minutes / 1 hour / 3 hours / 1 day / 2 days / 1 week / 1 month / 1 year), calendar ranges (today, yesterday, this week, last week, this month, last month, this year, last year) and custom ranges. The default is the last week.

### Metric cards

Four cards show spend, request count, token volume and cache hit rate. Each carries a sparkline and the change compared with the previous period.

### Charts

- **Spend ($/day)**: daily spend
- **Requests (calls/day)**: daily request count
- **Token breakdown**: daily input / output tokens
- **Prompt cache**: cached versus uncached tokens

### Attribution and filtering

The page provides three leaderboards — Top models, Top API keys and Top applications — to help you see where spend is concentrated. The filter bar supports multi-select filtering by model, API key and application, and the charts and leaderboards update accordingly.

### Export

Click **Export CSV** to export the data for the current time range and filters, including date, request count, spend, input tokens and output tokens. The file carries a UTF-8 BOM and opens directly in Excel. If the current range has no data, a message says there is nothing to export.

## Logs

The **Logs** page records each of your requests and its cost.

The page has three tabs at the top — **Request log**, **Midjourney** and **Async tasks** — addressed as `?tab=log`, `?tab=midjourney` and `?tab=task`, so you can bookmark or share a tab directly. **Request log** is the default. In an organization context only **Request log** is available; the other two tabs are hidden. Everything below describes the **Request log** tab.

### Histogram

The **Request volume** histogram above the list aggregates request count, spend and tokens over time, and can be expanded or collapsed.

### Columns

The default columns are time, group, API key, application, type, model, modality, status (finish reason), latency, throughput, input, output and cost. **Column settings** in the top right lets you show or hide columns freely.

Notably:

- **Modality** indicates which kind of endpoint the call used, such as chat, completions, Responses, embeddings, rerank, moderations, image generation / edit / variation, speech synthesis / transcription / translation, realtime, or video.
- **Status (finish reason)** indicates why the generation ended: completed, length truncation, content filter, tool call or error.
- The `t/s` figure under **Latency** is output tokens divided by total generation time, representing generation speed.

### Filters

The filter bar supports several dimensions, and enumerated fields can be either included or excluded:

- API key name, model, application (choose from preset values or enter a custom value)
- Finish reason, modality
- Minimum spend (quota), minimum token count
- Request ID (this site's trace ID, exact match)

The time range picker matches the **Usage** page. The **Refresh / clear filters** button resets everything at once.

### Single-record details

Click any row to expand the details of that request, including the overview, request-level information (application, API key, request ID, upstream request ID, finish reason, whether it was streamed) and the cost breakdown. The cost breakdown itemises the billing components for each token type (input / output text, audio, image, reasoning, cache reads and writes) and their ratios, and shows the group ratio, the actual input / output prices, the raw charge and the final charge, distinguishing **per token** from **per request** pricing.

If full request / response retention is enabled for both the site and your account, the details also show the request and response body for that call; when it is disabled, past the retention period, or you are not allowed to view it, an explanatory message is shown instead. The retention switch only applies to requests made after it is turned on; historical requests are not backfilled.

### Look up by ID

Choose **Find by ID** under **More actions**, paste a log ID, and the matching record is located and opened directly.

### Export

**Export CSV** under **More actions** exports the logs matching the current filters.

## Accounting records

Top-ups, referral rewards and other accounting records do not appear in the request logs; both tabs live on the **Billing** page, reached from the user menu at the bottom of the sidebar:

- **Transactions** tab: balance changes and transaction details;
- **Monthly invoices** tab: view and download invoices summarised by month.

## Related

- [API keys and quota](/guide/tokens): splitting API keys per application makes usage and log attribution much clearer
- [FAQ](/guide/faq): how quota is calculated and common errors
