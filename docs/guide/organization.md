---
title: "Organizations"
layout: doc
outline: deep
lastUpdated: true
---

# Organizations

Organizations let you and your colleagues share one quota pool, shared API keys and a shared usage view, while setting a budget and allowed models per member. This page covers everything you can do with organizations in the interface.

::: tip Whether you can see organization features
Organization features are enabled site-wide. If the identity card at the top of the sidebar does not open a **Switch context** menu, the feature is not enabled on this site.
:::

## Personal context and organization context

The identity card at the top of the sidebar is the context switcher. Open it to switch between **Personal** and each organization you belong to:

- **Personal context**: uses your own quota and personal API keys. The lower half of the card shows your personal balance, usage progress and request count.
- **Organization context**: uses the organization quota pool. The card shows the organization name, your role badge, the member count and the pool balance (visible to owners and admins only; members see **Balance visible to admins only**). It also has a **Budget this period** row with your usage / cap for the current period and a progress bar, showing **Unlimited** when no budget is set for you; when the organization has an organization periodic budget, owners and admins additionally see an **Organization budget** row.

Switching is global: once you switch to an organization, everything you see and create on the **API Keys**, **Usage**, **Top up** and other pages belongs to that organization. Your choice is remembered, so a page refresh keeps you in the same context.

Select **Manage organization** below the current organization in the switcher to open the organization management page; you can also open the **Organizations** entry in the sidebar directly.

## Roles and permissions

There are three roles in an organization:

| Capability | Member | Admin | Owner |
| --- | --- | --- | --- |
| Call models against the organization quota pool | ✅ | ✅ | ✅ |
| Create and manage their own organization API keys | ✅ | ✅ | ✅ |
| View organization API keys (all) | — | ✅ | ✅ |
| View organization usage | Own (subject to settings) | Everyone | Everyone |
| Invite members, create member accounts | — | ✅ | ✅ |
| Change member roles, remove members | — | ✅ | ✅ |
| Set member budgets and model allowlists | — | ✅ | ✅ |
| Edit organization profile and settings | — | ✅ | ✅ |
| Top up the organization, transfer personal quota in | — | ✅ | ✅ |
| View the audit log | — | ✅ | ✅ |
| Transfer ownership, delete the organization | — | — | ✅ |

Admins cannot act on one another, and you cannot change your own role or remove yourself. The interface hides entry points you are not allowed to use, and the server validates permissions again.

## Creating and joining an organization

### Creating an organization

Click **Create organization** at the bottom of the context switcher and submit an organization name. On success you are switched to the new organization automatically and you are its owner. The number of organizations an account may create is limited by the site.

### Joining by invitation

Open the **Organizations** page in the personal context and you will see **My invitations**:

- **Direct invitation**: an admin named you by username or email, so the invitation appears in the list directly with the inviter and the expiry time. Click **Accept** to join or **Decline** to void it.
- **Invitation link**: paste the invitation link or code you received into the **Join an organization with an invitation link** box and click **Join**.

## Overview

The **Overview** tab of the organization management page shows the organization avatar, name and your role, followed by these cards:

- **Quota pool balance**: the organization's currently available quota. Visible to owners and admins only.
- **Member count**
- **Organization budget this period**: quota used / cap for the current period, with the period and the next reset time underneath; it shows **Unlimited** when no organization budget is set. Visible to owners and admins only.
- **Created at**

Owners and admins can also click **Transfer quota in** on the balance card to move their own personal quota into the organization pool.

::: warning Transfers cannot be undone
Transferring personal quota into the organization pool is one-way and cannot be returned to your personal account. Your current personal balance is shown before the transfer, and the amount you enter cannot exceed it. This feature may be disabled by the site.
:::

## Member management

Member management lives on the **Settings** page (user menu), under **Members** in the organization group: the member list with user, role and join date; admins additionally see a **Budget** column and an actions column. The search box filters the current page by username, display name or email.

The admin view also has **Members / Invitations** segments at the top, the latter listing pending invitations.

### Inviting members

Click **Invite member** and pick a method:

- **Direct invitation**: enter an email or a username (one of the two). The recipient confirms it under their own **My invitations**, and it can only be used once.
- **Link invitation**: generates an invitation link that is copied to the clipboard on creation. You can set a **Maximum uses** (enter 0 for unlimited); any signed-in user can join with the link.

Both methods let you specify the role on joining (member or admin) and choose between never expiring and a specific expiry time.

The **Invitations** segment shows the invitee, role, status, use count and expiry of pending invitations. Link invitations can be copied again, and any pending invitation can be **Revoked**.

### Creating accounts for others

If someone does not have a site account yet, an admin can click **New member** and fill in a username and password (both required) plus display name, email and role; the account becomes a member of this organization immediately. The role you choose cannot be higher than your own. After creating the account you can set a budget for it or create organization API keys on its behalf.

### Changing roles and removing members

A member row's actions let you change their role. Removing a member requires a second confirmation, and **that member's organization API keys are disabled**.

### Member budgets and model allowlists

Click the budget value on a member row (or **Budget and allowlist** in the actions) to open the settings panel:

- **Periodic budget**: choose **No budget limit**, or set a daily / weekly / monthly cap. Once set, the panel shows the member's usage for the current period, and the list presents it as **used / cap · period**, highlighted in red when exceeded.
- **Model allowlist**: select the models this member may call. **Leaving it empty means the organization default applies, that is, no extra restriction.**

### Organization periodic budget

Besides capping each member individually, admins can set one combined cap for the whole organization: open **Budget** in the organization group of the **Settings** page, choose **No budget limit** or enter a daily / weekly / monthly cap, then click **Save**.

- The organization budget counts the **combined spend of all members within the period**. Once it is used up, every request in the organization is rejected until the next period starts.
- Both layers apply at once: a request is checked against your member budget first and against the organization budget next, and it only reaches the model when both still have room. Whichever layer is exhausted causes an immediate rejection at no cost.
- Changing the budget period resets the usage counter for the current period; switching back to **No budget limit** leaves only the per-member budgets in effect.
- Current usage is shown on the **Organization budget this period** card of the **Overview** tab and on the sidebar identity card; both are visible to owners and admins only.

## Organization API keys and top-ups

Once you switch to the organization context, the **API Keys** page manages organization API keys: members can only see and manage the API keys they created, while owners and admins can see every API key in the organization. Organization API keys are created exactly as personal ones are — see [API keys and quota](/guide/tokens).

Organization API keys draw on the organization quota pool and are subject to your member budget and model allowlist.

::: warning API keys follow membership
When a member is removed or leaves the organization, their organization API keys are disabled; when an organization is deleted, its API keys stop working too.
:::

Opening the **Top up** page in the organization context — from the balance row at the bottom of the sidebar — shows a notice at the top that the top-up will go into the organization quota pool:

- owners and admins can top up online or with a redemption code, and everything is credited to the organization pool;
- ordinary members get no top-up entry point in the sidebar balance row; opening the top-up page directly (`/panel/topup`) only shows a notice that **only owners and admins can top up the organization**.

Redemption codes may be restricted to one kind of account: an **Any** code can be redeemed by both personal accounts and organizations, an **Organization only** code can only be redeemed into the organization quota pool from the organization context, and a **Personal only** code can only be redeemed into a personal account. Redeeming in the wrong context returns *this redemption code can only be used by an organization account* (or *by a personal account*); the code is not consumed and stays usable once you switch to the right context.

## Usage

The **Usage** tab shows the organization's spend in detail:

- the time range can be today, the last 7 days, the last 30 days or a custom start and end date;
- three cards at the top show total spend, request count and tokens for the selected range;
- below that, tables break the data down **by member / by API key / by model**, listing request count, quota consumed, and input and output tokens; click a column header to sort;
- the dimension switcher also has **by date**, showing a daily trend chart that can toggle between request count, spend and tokens;
- click a member in the **by member** table to drill down into that member's API key details.

How much ordinary members can see depends on the **Members can view everyone's usage** switch in the organization settings: when it is on, members can view everyone's usage; when it is off, they only see their own data and the member filter is not shown.

After switching to the organization context, the organization usage panel on the **Dashboard** also offers **Export CSV**, which exports the aggregated organization usage for the last 7 days. General notes on logs and usage are in [Usage and logs](/guide/usage).

## Organization settings

Organization settings live on the **Settings** page (user menu). The organization group is split into **General** (organization profile), **Members**, **Budget** (organization periodic budget) and **Danger zone**, all visible to admins and above; the selector at the top switches between the organizations you manage. The organization periodic budget is described under [Organization periodic budget](#organization-periodic-budget) above.

### Organization profile

- **Organization name**, **Avatar URL**: how the organization appears in the switcher and on the overview page.
- **Members can view everyone's usage**: when on, ordinary members can view everyone's usage; when off, only their own. When unset, the site default applies.
- **Retain the organization's API request and response bodies**: a switch that decides whether request and response bodies for organization API key calls are retained for later troubleshooting; it is hidden when the site does not allow retention.

Click **Save** to apply changes.

### Danger zone

- **Leave organization**: any member may leave voluntarily and returns to the personal context; **your organization API keys will be disabled**. An owner must transfer ownership before leaving. Ordinary members have no settings page — use **Leave organization** at the top right of the **Overview** tab instead.
- **Transfer ownership** (owner only): pick one of the organization's admins as the new owner; after the transfer you are demoted to admin. If the organization has no admin yet, promote a member to admin first.
- **Delete organization** (owner only): you must type the organization name exactly to confirm. After deletion the organization and its API keys are disabled, and the action cannot be undone.

## Audit log

The **Audit log** tab is visible to owners and admins only. It records key operations in the organization in reverse chronological order, including time, actor, action and details. You can filter by action keyword or actor ID. Member management, invitation handling, quota transfers and top-ups, organization deletion and similar operations all leave a record.

Invitations that have been accepted, declined, revoked or expired no longer appear in the **Invitations** segment; you can trace them in the audit log.
