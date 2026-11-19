---
title: "Turning on supply routing"
layout: doc
outline: deep
lastUpdated: true
---

# Turning on supply routing

Supplies route each public model to the channels that supply it, in tiers, with pause rules and failover. New installations start with them on. Installations upgraded from an earlier release keep their setting: until the `ModelSupplyEnabled` option is on, requests are routed by the channels' model lists and model mappings exactly as before, the supply pages only show what would happen, and the providers page cannot order the channels.

Turn it on once you have checked what the migration changes on your own data. Every step below is done as the root user; the requests are sent from the browser's developer console on your site while you are signed in, so they use your session.

```js
// Helper used in the steps below.
const api = (method, path, body) =>
  fetch(path, { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) }).then((r) => r.json());
```

## 1. Back up the database

Take a backup you can restore (SQLite: copy the database file while the gateway is stopped; MySQL or PostgreSQL: a dump). The migration writes supplies, catalog rows and prices.

## 2. Dry run

```js
const report = (await api('POST', '/api/model_supply/migrate', { dry_run: true })).data;
```

Nothing is written. Read the report before going on:

| Field | What to check |
| --- | --- |
| `supplies_planned` | How many supplies will be created, one per channel and model. |
| `new_models`, `new_aliases`, `status_changes` | Catalog rows that will be added or published. A name a channel lists becomes visible to clients once it is published. |
| `billing_changes`, `price_copies`, `price_conflicts`, `plus_mappings` | Requests that will be priced by the public model instead of the upstream name. Fix conflicting prices first. |
| `unresolved_wildcards`, `skipped_entries`, `duplicate_entries` | Channel entries that will not become supplies. Fix them on the channel or accept the loss. |
| `policy_gaps` | Routing policies whose target models will have no supply. |
| `routing_differences` | Per user group, models that will be reached differently. Each one should be expected. |

Run the dry run again after each fix until the report shows only changes you want.

## 3. Migrate and switch on

Turn on **Route by supplies** under Platform admin → Gateway settings → Allocation → Supply mode and cost. Before you save, the switch shows how many supplies will be created and how many models would have no usable supply after the switch. Saving migrates first (only what is still missing), then switches on. The API does the same:

```js
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'true' });
```

What it migrates should match the last dry run.

## 4. Check

- `GET /v1/models` with a few real API keys lists the same models as before, apart from what the report named.
- On a model's Supplies tab, the request preview shows which channel the next request goes to. Prefer the preview over test requests: a request with an invalid key pauses the channel under the default credential rule.
- Watch the providers page and the logs for the first hour.

## Switching back

Turn the same switch off and save, or:

```js
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'false' });
```

Routing returns to the channels' model lists at once. The supplies stay, so switching on again needs no new migration; run a dry run anyway if channels changed in the meantime.
