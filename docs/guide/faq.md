---
title: "FAQ"
layout: doc
outline: deep
lastUpdated: true
---

# FAQ

## Quota and billing

**What is quota, and how does it convert to money?**

Quota is the site's internal accounting unit: 500,000 quota = $1. The interface usually shows US dollar amounts directly.

**How do I check what a request was charged?**

For usage-based models, cost = input tokens × input unit price + output tokens × output unit price, with input and output counted separately; for per-request models, a successful call is charged a fixed per-call price. Expand a record on the **Logs** page to see the actual input price, output price, group ratio and final cost for that call. The full rules are in [Billing and pricing](/guide/billing).

**My account quota is sufficient — why does it say quota is insufficient?**

Check the API key's own quota. API key quota and account quota are calculated separately: an API key quota only caps the maximum usage of that single key, and if either one is insufficient the request is rejected.

**Why do I see a different price than someone else for the same model?**

Price = model unit price × group ratio, and the ratio differs per group. Switch groups in the top right of the **Available models** page to see the price for that group.

## Request errors

**It says there are no available channels.**

There is no channel in your current group that can serve that model. Confirm on the **Available models** page that the model is available in your group (enable **Show available only** to filter), or switch the API key to a group that has it.

**Error: "The current group is at capacity, please try again later".**

The upstream returned a rate limit or a temporary outage; retry shortly. Contact the site administrator if it persists.

**A third-party client reports `Failed to fetch`.**

- Check that the API address and API key are correct;
- Check whether HTTPS is enabled — browsers block HTTP requests issued from an HTTPS page;
- Some clients need the address set to the root domain, with no extra path appended.

**A model shows "Price not configured" — can I use it?**

Not reliably. When the site has not configured a price for a model, calls may be rejected outright; use a model that has a price on the pricing page instead.

## Organizations

**It says the organization member budget is exceeded.**

An admin has set a periodic budget for you (daily / weekly / monthly). Once usage reaches the cap for the period, your organization API keys are rejected until the next period resets or an admin raises the budget. Your current usage is shown on the organization's **Members** page.

**Can I still use my API keys after being removed from an organization?**

No. After leaving or being removed from an organization, your API keys under that organization are disabled; your personal API keys and personal quota are unaffected.

**Can personal quota be transferred to an organization?**

Yes. Click **Transfer quota in** on the organization's **Overview** page to move personal quota one-way into the organization quota pool. **The transfer cannot be undone**, so confirm the amount before submitting.

**How do I review the organization's spend?**

The organization's **Usage** page breaks spend down by member, API key, model and date; whether ordinary members can view everyone's usage is controlled by the organization settings.
