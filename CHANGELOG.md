# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- While supplies route requests, saving a channel now carries edits of its model list and mapping
  to the supplies made from that list (when the channel was added, by the drift check or by the
  migration): models the edit added are supplied to the public models of the same name, a new
  mapping changes the name a supply sends upstream, and a model taken off the list has its supply
  withdrawn (adding it back supplies it again). Supplies added by hand or from the model library,
  and supplies whose upstream name was changed on the model page, stay as they are. The save
  message says what changed.
- The channel panel lists what needs attention on the channel (auto-disabled, no key, low balance,
  models added or removed upstream) above its tabs, each with its action, so the reason no longer
  hides behind the icon of a narrow row.

- Adding models from the model library now finds the channels that can supply a model even when
  models.dev does not know their spelling. Upstream names are compared word by word after taking
  off platform decoration (vendor and region prefixes, "@default", "-v1:0", "." versus "-"). Under
  each selected model the sheet lists the channels as exact, platform spelling or to confirm, with
  the sure ones ticked; variants such as "-mini" or "-thinking" are not matched, and ":free" tiers
  are offered unticked. Confirming or unticking a channel is remembered for that channel only.
- Allocation policy: a new Allocation page under Gateway orders providers once for every model
  while supplies route requests. Tiers are tried first to last; providers in one tier share it by
  weight, and the channels of a provider share its weight equally unless a channel is placed on
  its own. Providers that are not placed are tried after every tier. Models follow the policy
  without any setup; a model can be set by hand on its Supplies tab (or by renumbering one of its
  supplies there) and restored to the policy later. The first time supplies route requests the
  policy is built from the channels' current priorities and weights, and models whose routing it
  cannot reproduce are set by hand, so no traffic moves. While supplies route requests the
  channel panel no longer edits a channel's priority or weight, and the new channel sheet no
  longer asks for usage. `GET` / `PUT /api/allocation_policy` and
  `POST /api/model_supply/allocation` expose the same.
- A default supply mode and batch actions for supplies, so models do not have to be set up one
  by one. Platform settings → Gateway has "Default supply mode" (balance or failover). A model's
  mode switch gains "Follow default", which new models use; a model can still pick its own mode.
  On the models page, the batch bar has a "Supplies" menu: set the mode of the selected models,
  or add / remove one channel as a source of all of them (the upstream name is filled in and
  priority and weight follow the channel). APIs: `POST /api/model_supply/mode` and
  `POST /api/model_supply/batch`. Existing models on "balance" move to "Follow default" once;
  the default starts as balance, so routing does not change.
- "All groups" for channels in the admin UI: new channels no longer ask for groups and serve every
  user group, including groups created later. The group list under Groups & routing and the batch
  "Groups" dialog start with "All groups", which locks the single groups while it is on; turning
  it off selects every current group. Channels open to all groups match any group filter, count
  as peers of every channel of the same provider under "Usage", and show as "All groups" on the
  model's channels tab. Existing channels keep their groups.
- Channels open to all user groups: a channel whose group is `*` serves every enabled user group,
  including groups created later, without editing the channel. Creating, deleting, enabling or
  disabling a user group rebuilds the routing table; the channel list's group filter, the loss
  warning, the supply migration report and batch "add group" all treat `*` as every group, and
  saving a channel trims and de-duplicates its group list (`vip,*` is stored as `*`).
- New channels attach themselves to public models: while supplies route requests, a channel
  created from the new channel sheet gets a supply for every model it lists that is a public
  model in the catalog, at the priority and weight chosen under "Usage", and the confirmation
  says how many. The daily model check does the same for new upstream models on channels with
  "Add new upstream models" on; names that match no public model still wait under New upstream
  models.
- Upstream models in the channel panel: the Models tab lists every model the upstream returned
  at the last model check, with the public model each name belongs to and whether the channel
  supplies it (in use, supply switched off, available, not in the catalog, not recognized). The
  list can be filtered by state, searched and refreshed with "Check now"; rows link to the
  model's supplies. `GET /api/channel/:id/upstream_models` returns the same data.
- Add models from the model library: the models page has an "Add from library" button next to
  "Create". It lists every library model with filters for vendor, endpoint, release window,
  open weights and "a channel can supply it"; pick models or every match, add them as drafts or
  published, and optionally supply them from the channels that list them. Each added model
  takes its metadata from the library and the lab's list price when it has no price yet; the
  daily sync keeps added models up to date from the library.
- Endpoints show as small icons with their name on hover instead of text badges, in the models
  table and the library.
- Models without a vendor prefix or canonical id get a vendor from their name (`claude-` →
  Anthropic, `grok-` → xAI, `glm-` → Zhipu, …).
- A model library: the daily models.dev sync now keeps every model it knows, one per model
  (deduplicated by `canonical_model_id`), together with how each platform names it (Vertex's
  `claude-fable-5-2@default`, Bedrock's `us.anthropic.…`, OpenRouter's `anthropic/…`). The
  library is not public. It lets the channel side recognise platform names: the model picker and
  the new-upstream-models page suggest `claude-fable-5-2` for `claude-fable-5-2@default`, and
  adding a supply fills the upstream name from the channel's mapping, the names the channel
  lists, or its platform's spelling (Vertex, OpenRouter, Vercel, Groq). Bedrock and Azure keep
  the public name because their channels translate names themselves.
- The drop on the home page can be played with. A tap on the surface lets a drop fall right
  there and its ripple names a few of the models it passes; pressing and holding keeps the drop
  swelling until it is let go, and a bigger drop reaches further. The automatic drip steps aside
  while someone plays and resumes after five quiet seconds. It is a small inline script on the
  static home page and does nothing under reduced motion.
- The home page at `/` is served as static HTML, rendered at build time for each language
  (English, Simplified Chinese, Traditional Chinese, Japanese) without the app bundle, so search
  engines and first-time visitors get the content at once. The server picks the language from
  `?lang=`, the `appLanguage` cookie (set whenever a language is chosen) or `Accept-Language`,
  fills in the site name and docs link, and adds canonical and `hreflang` links. The header no
  longer reflects the login state: it always offers "Console", and the panel sends visitors who
  are not signed in through login and back to the page they opened. A custom `HomePageContent`
  or footer still uses the app-rendered home page.
- New upstream models wait for an admin. The models page has a "New upstream models" tab
  listing the models the drift check found new on a channel that no supply on that channel uses
  yet, each with its channels and the public model it most likely belongs to. Attaching adds a
  supply on the chosen channels, at each channel's priority and weight; a public model that does
  not exist yet is created as a draft. Ignoring takes the model off the list. API:
  `GET /api/model_supply/pending`, `POST /api/model_supply/pending/attach`,
  `POST /api/model_supply/pending/ignore`. While `ModelSupplyEnabled` is on, the channel option
  that adds new upstream models automatically leaves them on this list instead, and the drift
  check no longer reports a model a supply on the channel already sends.
- The supply migration (`POST /api/model_supply/migrate`) recognises a public model listed
  under another name. A channel model without a catalog row that the curated seed lists as an
  alias, or whose name without its `vendor/` prefix is a public model (for example
  `anthropic/claude-opus-5` for `claude-opus-5`), is supplied under that public model, and the
  listed name becomes its alias so requests for it keep working. The report lists these in
  `new_aliases`. Names that have a catalog row of their own, such as the vendor-prefixed rows
  catalog sync creates, are left as they are.
- The channel model picker list (`provider_models_list?detail=1`) returns, for each upstream
  model, the canonical name to publish it under (`canonical`) and how it was found
  (`canonical_source`: a catalog alias, a seed alias, or the name without its `vendor/` prefix;
  empty when there is none).
- Console endpoints for the model supply pages. `GET /api/model_supply/preview?model=&group=`
  shows in which order a request for a public model would try its supplies, tier by tier, with
  each supply's state (available, paused until, waiting for its recovery test, cooling down,
  disabled, channel disabled), its share of the tier's traffic, and which supply takes the next
  request when that is certain; it also works before `ModelSupplyEnabled` is on and says so.
  `GET /api/supply_pause/rules` returns the pause rules in effect and the built-in defaults, and
  a channel's own rules with `channel_id`. Channels can carry their own pause rules
  (`pause_rules`), tried before the site-wide ones; invalid rules are refused on save, and
  saving a channel without the field keeps them. The supply list also returns each channel's
  groups. With `ModelSupplyEnabled` on, the public model list (`/api/model_info/`, used by the
  pricing page) shows published models only.
- Pause rules for failing channels, on by default (`PauseRulesEnabled`). When a request fails,
  the first matching rule decides what happens to the channel: an exhausted quota or balance
  (402, `insufficient_quota`, `billing_not_active`, "credit balance is too low" and similar)
  pauses the whole channel for 30 minutes, a rejected key or disabled account for an hour; a
  rate limit cools the model down until the upstream's reset time, or for `RetryCooldownSeconds`;
  a missing model is paused for `ModelNotFoundCooldownSeconds`; `RetryCooldownPerStatus` still
  applies; and a 5xx pauses the model for 2 minutes on the third failure within a minute. The
  request that hit the failure still moves to the next channel. Pauses of a minute or more are
  stored and shared between nodes, and when one ends with a recovery test, the channel resumes
  only if a test request passes; otherwise the pause doubles, up to 6 hours. A balance refresh
  that finds money ends a quota pause early (refreshing a channel's balance by hand after a
  top-up resumes it at once), and the scheduled balance refresh now pauses a channel with an
  empty balance instead of disabling it. Admins get the new "channel or model paused", "paused
  channel or model resumed" and "request switched to a backup channel" notices (the last at most
  once an hour per model and channel). Rules can be replaced as JSON in `PauseRules`, and
  `/api/supply_pause` lists, adds and removes pauses. With `PauseRulesEnabled` off, failures are
  handled as before (automatic disabling, per-status cooldowns, model-not-found quarantine).
- Channels are organized by provider. The channel page lists the providers that have channels
  on the left (with a dot when one needs attention and the channel count) and a search box that
  finds channels across providers by name, model, tag, key suffix or host. The selected provider
  shows what needs attention (auto-disabled channels, missing keys, low balance, upstream model
  drift), one traffic chart with a line per channel over 24 hours, 7 days or 30 days, and the
  channel table with priority and weight, requests, success rate, average latency, balance or
  subscription usage and an enable switch. The table filters by status and tag, selects rows for
  batch enable, disable, test, group, priority and weight changes and delete, and offers
  tag-wide actions when a tag is picked.
- Clicking a channel opens a side panel whose tabs save on their own: models (per-model
  requests, success rate, latency and first token time; model list and mapping), connection
  (base URL, proxy, masked key and replacement, OAuth account, subscription windows), groups
  and routing (groups, and the channel's place next to the other channels of the same provider:
  backup, share or first, with the priority and weight that saves), request rewriting, monitoring
  (test model, balance alert, drift) and advanced fields. `/panel/channel/<id>` opens the panel
  directly; older `?view=settings`, `?tab=config` and `?channel_id=` links land on it.
- New channels are created in a side sheet: provider, name, allowed models (all by default, or a
  list with `*` prefix wildcards), blocked models (removed from the list and added to the drift
  ignore rules so they are not added back), model mapping, the provider's auth method and key
  with a link to where the provider issues keys, groups and usage. Test checks the connection
  before saving; Save and test creates the channel and then tests it. Several keys, one per
  line, create one channel each. `/panel/channel/new` opens the sheet. `POST /api/channel/` now
  returns the created ids.
- Model supplies, backend groundwork (no change to request handling yet). A new `model_supplies`
  table records which channel supplies a public model, under which upstream name, with its own
  priority, weight and on/off switch; admins manage them through `/api/model_supply`, whose list
  shows each supply's channel name, status, type and tag. Root can
  run `POST /api/model_supply/migrate`, a dry run unless the body says `{"dry_run": false}`, which
  turns the channels' model lists and model mappings into supplies and reports what changes: new
  catalog rows, status changes, prices copied from upstream names and conflicts between them,
  requests that will be priced by the public model instead of the upstream name, `+` mappings,
  wildcards expanded from the last upstream model list or left unresolved, duplicates, routing
  policies whose models have no supply, and per-group routing differences. Catalog rows gain
  `status` (draft, published, retired) and `supply_mode` (balance, failover); hiding a model keeps
  `status` in step until the migration runs, and saves that leave the two fields out keep them.
- Routing through model supplies, behind the new `ModelSupplyEnabled` setting (off by default;
  turn it on after running the migration). While it is on, the channels' model lists and model
  mappings no longer route: each public model reaches the channels that supply it, in tiers by
  the supplies' priority (highest first), and within a tier `balance` models split by supply
  weight while `failover` models take the heaviest supply. The supply names the upstream model,
  so `model_mapping` and its `+` prefix no longer apply, and requests are billed by the public
  model's price rather than the upstream name's. `status` decides what clients see: published
  models are listed and callable, retired ones stay callable but are no longer listed, drafts
  answer model not found, and aliases keep working without being listed. The request log
  records the supply in `metadata.supply_id`. Switching the setting rebuilds routing at once,
  in both directions.
- Organization routing policies (backend): with the new site option
  `OrganizationRoutingPolicyEnabled` (off by default), organization owners and admins manage
  routing policies that only their organization's API keys can use, through
  `/api/org/:id/routing_policy` (list, create, update, enable or disable, delete, stats, "Try it"
  and the logprobs probe). An organization policy may reuse a platform policy's name and then
  replaces it for that organization; personal keys and other organizations keep the platform
  policy. Organization policies may only use models the organization's group can route to and the
  catalog does not hide, checked when saved and again per request: a target hidden later sends the
  request to the fallback (reason `target_unavailable`), and a hidden fallback answers model not
  found. "Try it" and the probe are billed to the organization and logged with `routing_try`, which
  keeps them out of the stats. Changes are written to the organization audit log, each organization
  is capped by `OrganizationRoutingPolicyMaxCount` (default 20), and system admins can list an
  organization's policies with `/api/routing_policy/?organization_id=N`. Logs now carry
  `routing_policy_id`, and session pins are keyed by policy id, so conversations already pinned
  under a platform policy are classified once more after upgrading.
- Organization routing policies in the console: organization owners and admins manage them under
  Organization, Auto routing in the sidebar (shown while the site option is on), with the same
  list, policy panel, "Try it" and statistics as the platform page; the model pickers list the
  models the organization can use (`GET /api/org/:id/routing_policy/models`), and "Try it" says it
  is billed. Platform settings, Gateway, Auto routing gains the site option and the
  per-organization cap. The platform Auto routing page gets a Scope filter: Organizations lists
  every organization's policies (`GET /api/routing_policy/?organization_id=-1`), marks ones that
  override a platform policy, and opens them read-only so admins can only switch them off. The
  routing guide covers organization policies in all four languages.
- Personal quota warning settings: Settings, Preferences has a Quota warnings card where users
  turn the low-balance and used-up emails off, or set their own warning threshold instead of the
  site default (and switch back to it). The site switch still wins: when it is off nobody gets
  the emails. The settlement reads the setting from the same user row it already loads, and a
  changed setting applies at once instead of after the 24-hour "already sent" window.
- Bedrock channels can use an IAM role instead of a long-lived key with Bedrock permissions. Put
  the role ARN in the fourth field of the channel key (`Region|AccessKeyID|SecretAccessKey|RoleArn`),
  or `Region|AccessKeyID|SecretAccessKey|SessionToken|RoleArn` when the source key is itself a
  temporary credential. The gateway assumes the role through STS, caches the temporary credentials
  per key and proxy, and renews them five minutes before they expire, so the access key only needs
  `sts:AssumeRole`. A key with the wrong number of segments is now logged as invalid instead of
  being read as an empty configuration.
- Auto routing policies: a policy is a virtual model name (such as `auto/coder`) that routes each
  conversation to one of several real models. On the first user message the gateway asks a small
  decision model which of the policy's labels fits the request and forwards the conversation to
  the model mapped to that label; below the confidence threshold, on any decision failure, or when
  the first message has no text, the fallback model answers, so routing never rejects a request.
  The choice is pinned per session (session headers, Claude Code metadata, `prompt_cache_key` or
  the prompt prefix) so a conversation never switches models and the upstream prompt cache keeps
  hitting. Policies work on the Chat Completions, Claude Messages, Responses and Gemini entries,
  appear in `/v1/models` and the model catalog (marked as auto routing, billed by the target
  model), and can be used in token and organization allow lists, which are checked against the
  policy name. The routed request is billed at the answering model's price; the decision call is
  billed once per session at the decision model's price as its own log entry (a new setting turns
  this off). Routed responses carry an `X-Modeltaps-Route` header, consume logs record the policy,
  label, confidence and reason, logs can be filtered by policy, and the Playground shows which
  model answered. Administrators manage policies under Gateway → Auto routing, with templates and
  a "Try it" panel that runs only the decision step. See the Auto Routing guide in the docs.
  Classification skips the context coding agents inject (Claude Code `<system-reminder>` blocks,
  Codex `<environment_context>` and `AGENTS.md` messages), and Claude Code side requests such as
  session titles are not classified: before the session is routed they go to the fallback model.
  Fallbacks after a decision failure or an empty first message are not pinned.
- Each routing policy shows its effect over the last 7 or 30 days, from the consume logs: requests
  by decision reason, hits per label with the model and spend, the fallback share, the average
  decision time, what the decision calls cost, and the estimated savings against sending everything
  to the fallback model. The policy list shows the 7-day request count and fallback share, and
  "Try it" can replay a real first message from the logs when request detail logging is on.
- Routing policies can score the decision with logprobs: the labels are sent as lettered options,
  the decision model answers with one letter, and each label's probability is read from the
  model's output probabilities, so the confidence threshold compares a real probability instead of
  a number the model reports about itself. A new per-policy "Scoring" setting chooses auto (the
  default for new and existing policies), logprobs only, or self-reported only. Auto uses logprobs
  and, when the decision model does not return them on a channel (Claude, reasoning models and most
  aggregator gateways do not), decides that request again with self-reported confidence and keeps
  using self-reported confidence on that channel for 24 hours; logprobs only sends the request to
  the fallback model instead. A "Check support" button in the policy form tests whether the
  decision model returns logprobs. The `X-Modeltaps-Route` header carries `scoring=`, consume
  logs record the scoring method and, with logprobs, every label's probability, "Try it" shows
  both, and the policy statistics break requests down by scoring method with the average top
  probability.
- Image generation requests keep the SVG and GIF options (`complexity`, `frame_count`,
  `frame_duration_ms`, `svg`) when the channel does not pass the body through, and the `svg` and
  `svg_usage` fields of the response (including the synthesized streaming `completed` event) are
  returned to the client instead of being dropped. Codex image channels forward the same options
  and return SVG and GIF results with the right MIME type instead of `image/png`. The default
  prices include `gpt-image-2.6-flare` and `gpt-image-2.6-sunburst`, priced like the 2.5 models.
- The log page lets users filter their own logs by request IP (one or several values, or exclude
  some), also for the CSV export, the histogram and the total spend. Logs of administrator actions
  on the account never match an IP filter, so the filter cannot be used to find out which IP an
  administrator acted from.
- AWS Bedrock maps `claude-fable-5-2` to its inference profile (`us.` in the US and Canada,
  `global.` elsewhere), and new Bedrock channels list it among their default models. The default
  prices include `claude-fable-5-2` ($8 / $40 per million tokens, cache writes $10 for 5 minutes
  and $16 for 1 hour, cache hits $0.20), and new Anthropic channels list it among their default
  models.

### Changed

- The model panel is wider (tabbed sheets use the wide size), so the Supplies table no longer
  cuts off channel and upstream model names. Below the table, "Channels that can also supply
  this model" lists enabled channels that list the model and are not a supply yet; "Add" makes
  one a supply with the suggested upstream name and the channel's priority and weight.
- The channel menu names its checks apart: "Test several models" (connection and latency),
  "Capability check" (real requests that verify vision, tool calls and more) and "Compare upstream
  models", each with a line saying what it does. The Advanced tab shows the "other" field only
  when a provider that does not read it still holds a value, the no-streaming example uses the
  channel's own models, and the Responses conversion example shows the syntax instead of OpenAI
  model names.

- Providers list columns: "Requests" (was "Requests in 24 h") shows the count first and its trend
  line after it, left-aligned and set apart from the share column; channels that share with no
  other show "—" for share. Column headers explain themselves on hover in one short line.
- Channels that send to an address other than their type's own (relays such as an OpenAI-compatible
  gateway) show that host next to the provider name in the providers list, so two relays of the
  same type can be told apart.
- Call order: every position is now a group of channels that really share traffic. Saving splits
  a position whose channels offer no model in common into consecutive positions (routing is the
  same), so numbers, arrows and dragging all work on these groups. Each position has up / down
  arrows next to its number. Dragging a row shows the whole row; dropping on its top or bottom
  edge inserts before or after it and in the middle joins its group, with a line or a "share with
  this group" hint showing where it will land. Joining a group that offers no model in common
  places the channel right after it and says so.
- The providers page header is one toolbar row: search, a provider dropdown, status, an
  "N need attention" filter, conditions, the time range and columns. Attention items (low
  balance, auto-disabled, missing key, model changes) are tags on their channel's row with the
  action and a dismiss button, and "Refresh model lists" / "Test all" moved into the ⋯ menu. In
  the call order, channels are numbered by the group that really shares traffic (channels of one
  position that serve the same models), the number cell spans the group, and dragging a row onto
  the middle of another joins its group while dropping on its top edge inserts it before.
- The providers page is more compact: provider cards became filter chips next to the search, and
  the traffic chart is replaced by a small request line in each channel's row, with the range
  switch in the list toolbar. Every row shows its position number (the line joining a position
  is gone), and a channel that shares traffic names the channels it shares with.
- The providers page is one channel list. Provider cards at the top keep each provider's traffic in
  view and filter the list; below them every channel is listed in call order while supplies route
  requests: tried top to bottom, reordered by dragging or from the row menu, and a channel can share
  the position above it by weight (shares are shown only between channels that serve the same
  models). New channels go last. "New channel" opens a searchable list of providers first. The
  separate Allocation page is gone (its address now opens the providers page), and channels the
  order does not place are tried one at a time after it, oldest first, instead of sharing one
  last tier.
- The admin page that lists upstream connections is now called **Providers** in the sidebar and
  page title (it was **Channels**). The entries inside a provider are still channels; links that
  point to the page say "Providers" as well. Nothing else on the page changed.
- The models page lists catalog models and models a channel lists; models that only have a price
  no longer appear. Their prices stay, and adding the model from the library keeps its price.
- On upgrade, synced draft models nothing uses are removed from the catalog (kept: anything not
  a models.dev draft, locked rows, aliases and their targets, models with a supply, and names a
  channel lists or maps). They stay in the model library.
- The models.dev sync no longer adds models to the public catalog. It refreshes the metadata of
  models already there; new models go to the model library.
- Settings is one place again, as in 0.3.0. Settings in the account menu and a settings button
  next to the account row (it replaces the bell) open it, and the sidebar turns into its tree,
  laid out like the workbench and platform admin sidebars: the same brand row, a back row to the
  page you came from where the context switcher is, the picker of the organizations you manage
  (owners and admins) right under it, then General, Members & budgets and Danger zone of that
  organization and Personal settings. The page is titled with the item, and with the
  organization's name under organization items. Unsaved changes show as a dot in the tree.
  Picking an organization there no longer switches the active space. The organization menu no
  longer has an Organization settings item; Members & budgets stays there as a shortcut into
  Settings.
- The panel has one search, the search palette (⌘K), and every search button opens it. In the
  workbench, platform admin and settings sidebars it is an icon at the end of the brand row (the
  search row under the context switcher is gone), and the platform settings subnav's search box
  is a button for it. On a settings page the palette lists the page's cards under On this page
  and finds them by anything written on them, collection rows included; picking one opens its
  group with the card highlighted. While typing it also lists every group of Settings. The in-
  page filtering of settings cards is gone. Every sidebar shows its collapse button only on
  narrow screens, where the rail collapses by itself and the page title has a button to expand
  it, and in the phone drawer; wide screens keep the rail expanded.
- The site announcement no longer opens a dialog on page load. While it is unread, the console
  shows one row above the page title with its first line, a button that opens the full text and
  a close button; reading or closing it hides the row until the announcement changes. The
  account menu has an Announcement item while the site has one, and an empty announcement shows
  nothing. The announcement is shown in the console only, no longer on the login page.
- The wordmark next to the logo is set in a rounded face (a nine-letter subset of Fredoka
  SemiBold, SIL OFL 1.1, inlined into the stylesheet) at a size that lines up with the mark. The
  header, sidebar, footer and static home page share one lockup. A custom site name keeps the UI
  font at the same size.
- The home page animation is now a drop: the mark gathers under a tap, falls onto a still
  surface and the ripple lights up the models on it ring by ring. It runs on CSS alone, so the
  static home page animates without the app bundle, and it holds still under reduced motion.
- The line under the home page headline now states what the gateway does. In English it names
  the OpenAI, Anthropic and Gemini protocols and says only the base URL changes; the Chinese and
  Japanese lines say all three mainstream APIs are supported and existing tools work as they are,
  without the developer term "base URL". The tap above the
  drop is drawn as a faint hairline.
- Every panel page uses one sidebar. Under the brand, a one-line switcher shows the current space
  (personal, the active organization, or platform admin for administrators) and switches between
  them; platform admin is no longer a separate sidebar with a back row, and the user menu and the
  workbench no longer carry their own admin entries. The workbench menu is dashboard, usage and
  logs, then a Develop group (API keys, models, playground, docs) and, for organization owners and
  admins, an Organization group (members and budgets, billing). The five API consoles are one
  Playground item with a modality switch on the page; analytics and per-user statistics are one
  Statistics item with a tab switch. The platform settings groups get a section list on the left
  of the page instead of sidebar entries. The bottom of the sidebar shows the account row (its
  menu has the balance, settings, appearance, language, platform admin, docs and log out) and the
  settings button. URLs are unchanged.
- New brand mark: a solid, rounded drop with an off-centre round hole replaces the stroked
  lowercase m. The mark is square-ish and filled, so it holds its weight next to the wordmark in
  the sidebar and header and stays legible as a 16 px favicon. The favicon, PWA icons, Open Graph
  image and the documentation site's logo and favicon use the same shape. It is drawn on whole-pixel
  grids at 12, 16 and 20 px and shown as pre-rendered bitmaps in the interface and as PNG
  favicons, so its edges stay smooth on standard-density screens.
- Platform, organization and personal settings are one kind of page with three kinds of cards.
  A form card saves everything in it, switches and its on/off switch included, from its footer
  (Undo and Save); nothing saves the moment it is clicked any more, and no card has several save
  buttons. Lists of objects (OIDC providers, payment gateways, the Telegram bot menu, chat links,
  login sessions) are collection cards whose rows open an editor that saves the item on its own.
  One-off actions (purging logs, monthly invoices, transferring or dissolving an organization)
  are action rows with a confirmation. Groups with unsaved cards get a dot in the section list,
  leaving the page with unsaved changes asks first, and pages with more than three groups have a
  settings search that also finds fields, hints and list entries in other groups.
- Platform settings are regrouped into Site, Sign-in & registration, Billing, Payments, Gateway,
  Logs & privacy, Notifications & integrations and Organization policy (root only); mail sending
  moves to Notifications & integrations and the provider icon library into Branding. Old group
  and section links (`relay`, `integrations`, `organization`, `?tab=`, `?highlight=` of a card
  that moved) redirect to the new place, and the command palette lists the groups and, while
  typing, every card. Gateway gets a supply pause rules card (on/off switch and custom rules
  as JSON), and the notification switches cover paused, resumed and failed-over supplies.
- Invite codes have their own page under Finance in platform admin (`/panel/invite_code`); the
  settings keep the "registration requires an invite code" switch with a link to it.
- Personal access tokens move into Sign-in & security, and the organization team budget joins
  the organization profile under General; members and budgets stay their own page.

- Announced official price changes take effect on their date without a release: `gemini-4-argon`
  moves to $4 / $20 per 1M tokens (cache $0.20) on 2026-12-01, and `gemini-3.8-flash` and
  `gemini-3.6-flash` to $1.50 / $7.50 (cache $0.15) on 2027-01-01. An hourly job updates stored
  prices that still equal the previous default; prices an administrator edited or locked are left
  alone.
- `gpt-5.5-pro` default price includes the long-context tier (over 272K input tokens: $60 / $270).

### Fixed

- While supplies route requests, a new channel now supplies the models it lists even when they
  had no catalog row yet. Their rows are created by the catalog sync that runs after the save, and
  the save tried to attach them before that, so on a fresh catalog a new channel ended up with no
  supplies and its models showed "no channel". The channel is now attached once the sync is done;
  the same goes for models an edit adds.

- AWS Bedrock channels in Canada (`ca-central-1`, `ca-west-1`) now call `claude-fable-5` and
  `claude-fable-5-1` through the `us.` inference profile, as they already did for
  `claude-fable-5-2`, instead of falling back to `global.`. Both Canadian regions are source
  regions of the US profile, so requests stay inside the US and Canada.
- The same applies to `claude-sonnet-4-6`, `claude-sonnet-5`, `claude-opus-5` and `claude-opus-5-5`:
  Canadian Bedrock channels now use the `us.` profile instead of `global.`. EU Bedrock channels
  call `claude-sonnet-5` through the `eu.` profile, which AWS added after the model launched,
  instead of `global.`, so requests stay in the EU.
- The allocation page worked out percentages over a whole tier, so channels that never serve the
  same model looked like they split traffic. Shares are now worked out only between entries of a
  tier that serve a model in common; an entry that shares none shows no weight, and each row names
  the entries it shares with. `GET /api/allocation_policy` lists each channel's supplied models.
- The supply request preview counted channels open to all user groups as outside the chosen group.
- Changing a channel's priority or weight reaches its supplies again. With supplies routing
  requests, edits on the channel (its panel, the batch actions on the providers page and the
  tag-wide setters) only changed the default for new supplies. Supplies that still have the
  channel's priority or weight now move with it; a supply given its own numbers on the models
  page is marked "Own setting", stays where it is and can be set to follow the channel again.
- Models synced from models.dev no longer take the hosting platform as their vendor: Claude,
  Grok or Llama served on Vertex showed as Google Gemini, and Grok or Phi on Azure as OpenAI. The
  vendor now comes from the lab named in the model's canonical id and stays unknown when there
  is none; the next sync corrects rows written the old way.
- With `ModelSupplyEnabled` on, the models page still counted a model's channels from the
  channels' model lists, so a model served only through supplies showed "No channel" and one a
  channel merely listed looked routable. The admin catalog (`/api/model_info/catalog`) now lists
  the channels of a model's enabled supplies, with the supply's priority and weight, and says
  `supply_routing`; the page counts those, shows a model as visible to users only when it is
  published (its visibility switch follows the status), and points the channels tab to the
  Supplies tab.
- Applying the catalog seed on a fresh install changed nothing for aliases, because it only
  marked alias rows that already existed. It now creates the missing alias rows (reported as
  `created_aliases`, replacing `missing_aliases`), so channels that list those upstream names get
  the canonical name and mapping in the same run.
- A request billed under an upstream alias that has no price of its own (the usual case after a
  channel maps `claude-opus-5` to `anthropic/claude-opus-5`) was rejected as unpriced. Billing now
  falls back to the canonical model's price, as the catalog already promised for aliases; an alias
  with its own price keeps it.
- Saving a channel no longer resets its last test time, response time and creation time; the
  form does not carry them, and the full-row update wrote them back as zero.
- Channels that list a wildcard model such as `gpt-4o*` now send the requested model upstream
  (`gpt-4o-mini`) instead of the literal pattern. The pattern was also the name billing looked
  up, so with the default unpriced-model policy these requests were rejected with "has no
  configured price" before reaching the provider. A channel's model mapping may name the
  requested model or the pattern; the requested model's entry wins.
- The channel that failed last in a request is now cooled down as well, when retries are turned
  off or the retry budget runs out before the channels do. A rate-limited primary was otherwise
  picked again by the next request. Sessions held on their channel by strict session affinity
  are left where they are.
- A provider error that is not retried is counted once in the `provider_requests_total` metric
  instead of twice.
- Saving a setting now reports a database error instead of applying the value in memory only,
  where it looked saved until the next restart or sync.
- A request whose channel turns out to be out of quota or credit now moves to the next channel
  instead of failing. Some providers report an empty balance as a 400 (for example "Your credit
  balance is too low"), which ended the request on every channel type except Anthropic, even
  though the channel was then disabled. Any error that matches the automatic disable rules
  (exhausted quota or balance, rejected key, disabled account, a disable keyword) now retries on
  another channel whatever its status code and whether or not automatic disabling is on, and also
  when strict session affinity binds the session to that channel. A malformed request still
  returns its 400 without retrying.
- The scheduled recovery check now sends the "Channel re-enabled" notice when it puts an
  automatically disabled channel back into rotation; the notice type was listed in the
  notification settings but never sent. With several nodes running the check, only the node that
  re-enables the channel sends it. A recovery also clears the disable notice's de-duplication
  window, so a channel that fails again within five minutes is reported again.
- Claude models on Vertex AI channels now work. Requests were sent to Google's publisher path, so
  Claude models were not found there; Claude now uses the Anthropic publisher, and model IDs with
  a date (`claude-sonnet-4-5-20250929`) are converted to Vertex's `name@date` form while models
  without a date (`claude-opus-5-5`) are sent unchanged. Requests to `/claude/v1/messages` are
  forwarded as sent instead of being converted to the OpenAI format and back, so cache control,
  tools and other Claude fields reach Vertex unchanged; the premium service-tier fields stay
  stripped unless the channel allows them, and only the `anthropic-beta` values Vertex accepts are
  forwarded. The region can be `global`, `us`, `eu` or a specific region (`us` and `eu` use
  Google's multi-region hosts). A custom API address now needs six `%s` placeholders (host,
  project, region, publisher, model, action); the older five-placeholder form still serves Gemini
  models, and any other address is rejected with an error instead of being ignored. Vertex errors
  show the upstream message with the project ID masked instead of a fixed text, and an empty
  error list no longer crashes the handler. A Claude model mapped to a Gemini model still takes
  the Gemini conversion path.
- DeepSeek cache hits reported only as `prompt_cache_hit_tokens` on the OpenAI format are billed
  at the cache hit price instead of the full input price.
- Claude Code sessions are recognised again from `metadata.user_id` in the format current Claude
  Code sends (a JSON object encoded as a string), so Claude channel session affinity follows the
  Claude Code session instead of falling back to a hash of the cached prompt.
- Codex image channels no longer fail `gpt-image-*` requests with "Complexity 'medium' is not
  supported for this image model". The listed models (`gpt-image-1.5`, `gpt-image-2`, and the
  2.5 and 2.6 variants) are now sent to the ChatGPT backend's native images endpoints instead of
  through the Responses orchestration model, which filled in `complexity` on its own, so
  `complexity` reaches the renderer only when the client asked for it. The request is built from
  an allow-list of image options (no `extra_params` or end-user identifiers), usage is billed as
  before (with the image estimate when the upstream omits it), and `response_format=url` still
  returns a data URL. Channels with a custom base URL, and other image models, keep using the
  Responses path, which now drops `complexity` instead of forwarding a value the hosted tool
  rejects. `/v1/images/edits` on Codex channels also forwards `background`, `moderation`,
  `output_compression`, `output_format`, and the SVG and GIF options instead of dropping them.

### Security

- Image URLs in requests are no longer fetched from loopback, private-network or cloud-metadata
  addresses; redirects are re-checked on every hop.
- The Midjourney notify endpoint is disabled (the gateway never registers it and task state is
  synced by polling), and `/mj/image` only relays images: it reaches private addresses only on the
  task channel's own host.
- The up-front charge now covers an output estimate (the request's `max_tokens` or the new
  "Output Tokens Reserved per Request" setting, default 4096, whichever is smaller). Skipping the
  up-front charge for large balances is limited to unlimited tokens and records an in-flight
  reservation in Redis, so concurrent requests can no longer drive a balance far below zero or
  exceed a token's quota.
- Payment callbacks credit an order only when the gateway, currency and paid amount match it, and
  Stripe only credits paid sessions (asynchronous payment methods are credited on
  `checkout.session.async_payment_succeeded`, which the webhook now subscribes to).
- Relay request bodies are capped by `relay_max_body_mb` (default 128 MB).
- Email verification codes are invalidated after 5 wrong guesses and once used.

## [0.3.0] - 2026-11-04

### Fixed

- Image generation requests keep the `safety_identifier` field when the channel does not pass the
  body through: it is now forwarded upstream, and the value echoed in the response (including the
  synthesized streaming `completed` event) is returned to the client instead of being dropped.
- Quota reminder emails are checked after a charge succeeds, including the final settlement,
  where the balance actually crosses the threshold. Before, the check ran on the pre-charge
  estimate only, before the charge (a failed charge could still email) and not at all when the
  pre-charge was skipped for a large balance, so most crossings were never reported, while
  concurrent requests crossing together each sent one. Each warning ("running low", "used up") is
  now sent once per crossing, across instances with Redis and per instance without it, and again
  only after the balance has clearly recovered and drops again (at most one reminder a day while
  it stays low). The email no longer shows a negative balance. Organization keys skip the check
  before any extra read. A new switch in the quota settings, on by default, turns the reminders
  off together with the extra balance read at settlement.
- Clearing consume logs by hand in the settings no longer runs one unbounded `DELETE` inside the
  HTTP request, which held the busiest table for minutes on a large backlog and left the
  request details of the deleted logs behind. The cleanup now runs in background batches like the
  daily auto-delete, removes the matching request details, retries a failed batch a few times
  before giving up, and logs progress to the system log. Only one cleanup runs at a time per
  instance: a second click while one runs is told so, and the daily auto-delete waits instead of
  overlapping a long run.
- Malformed request bodies no longer crash the request with a `500` panic: content parts that are
  not objects or carry non-string `text` / `image_url` fields are skipped during token counting, an
  OpenAI `tool_choice` of type `function` without a function name returns `400` instead of
  panicking on Claude channels, and Ali, Baidu and Zhipu embeddings reject empty, non-string and
  token-array inputs with `400` instead of panicking or sending an empty input upstream. The panics
  happened after the quota pre-charge and left it unrefunded.
- A user's own usage total on the logs page is counted by user ID, like the log list, instead of
  by username: logs from before a rename are no longer left out, and the query uses the
  per-user index instead of scanning every log in the time range.
- Gemini Long Decode can be continued through the native non-streaming `generateContent` relay:
  the response keeps `candidates[].continuationToken` instead of dropping it. A
  `finishReason` of `CONTINUATION` now maps to `length` on the OpenAI-compatible endpoints and in
  the logs, instead of `content_filter` / `error`.
- A channel custom parameter `store` (such as `{"store": false}` to stop OpenAI from retaining
  chats) is now only sent to Chat Completions and Responses. Image, embedding, speech, moderation
  and legacy Completions requests on the same channel failed with `Unknown parameter: 'store'`.
- Sticky sessions for Claude Code, Gemini CLI and Codex account channels never matched: the
  session lookup asserted an interface the router's request context no longer satisfies, so every
  request was spread across accounts. They now stick as intended, also without Redis.
- Request fields that move the upstream to a pricier processing tier are no longer forwarded by
  default: OpenAI `service_tier` `priority` / `fast` / `ultrafast` on Chat Completions and Responses,
  and Claude `speed: "fast"` and `inference_geo: "us"` on the native Messages pass-through. The
  upstream charged the higher rate while the request was billed at the standard price. A new
  per-channel switch, "Allow premium processing tiers", forwards them again.
- Requests are billed at the processing tier the upstream reports: OpenAI `service_tier` (Fast /
  Priority 2x, Ultrafast 6x, Flex 0.5x), Claude `usage.speed` fast (2x) and `usage.inference_geo`
  `us` (1.1x). The tier and its ratio are recorded in the log details; the upstream cost follows
  the same factor.
- Model lists (`/v1/models`, `/claude/v1/models`, Gemini `models`) for organization API keys only
  show the models the member's allowlist permits, so clients and coding agents no longer offer
  models that are then rejected. An API key with the model limit on and no model selected now lists
  no models, matching the requests it rejects, and the key form no longer saves that combination.
- Requests for a model the API key may not use return `404 model_not_found` instead of a `503`
  "upstream at capacity" error that SDKs keep retrying.
- Organization member limits: an organization's own limit can now be unlimited even when the
  site default is not. `max_members` is null to follow the site default, 0 for unlimited, or a
  positive limit; existing organizations keep following the site default. Site admins set it from
  the organization list, which also shows each organization's members against its limit.
- Claude-protocol requests (`/v1/messages`, as sent by Claude Code) to OpenRouter and Vercel AI
  Gateway channels are forwarded to the gateways' Anthropic-compatible endpoints instead of being
  converted to Chat Completions and back. The conversion dropped the client's `cache_control`
  breakpoints on system prompts, tool definitions and recent turns, so prompt caching was lost
  through these channels; thinking blocks and beta headers now also pass through unchanged, and
  cache read / write tokens are billed from the upstream usage.

### Added

- The model catalog records whether a model has open weights, when it was released and when the
  upstream retires it, synced from OpenRouter (`hugging_face_id`, `created`, `expiration_date`),
  Vercel AI Gateway (`open-weights` tag, `released`) and models.dev (`open_weights`,
  `release_date`).
- `POST /api/channel/provider_models_list?detail=1` returns the upstream models grouped by model
  family (`claude-opus-4-5` and `anthropic/claude-opus-4.5` are the same family), marks the
  newest version of each family, adds catalog metadata and token prices, and ranks the families
  by this site's requests over the last 30 days, falling back to a built-in vendor order. The
  existing response without `detail` is unchanged.
- Subscription channels show their usage windows instead of a money balance: refreshing a Codex
  channel's balance (or the periodic refresh) queries its 5-hour and weekly windows, and Claude
  Code channels keep the windows Anthropic reports on each response. The channel list shows the
  used percentage per window with the reset times in a tooltip.
- Hourly channel metrics: attempts, successes, latency, time to first token and output speed per
  channel and model, counted in memory and written every minute, kept 30 days by default.
  `GET /api/channel/metrics` returns them; the log settings card turns them off or sets the
  retention.
- Channel options for the upstream model check: "Ignored upstream models" (exact names, `*`
  patterns or `re:` regular expressions) are never reported as new, and "Add new upstream models
  automatically" adds newly offered models to the channel with their prices and metadata.
- Account channel credentials (Codex, Claude Code, Gemini CLI, Antigravity) are refreshed in the
  background every 30 minutes when they expire soon, including idle and auto-disabled channels;
  failures are sent as a credential notice. The interval is set in the channel auto-disable
  settings (0 turns it off).
- Admin notifications can be turned on or off per kind (channel disabled / re-enabled, channel
  test, low balance, price changes, model list changes, model metadata, credential problems) in the
  new "Admin notifications" settings card, and each kind is limited to 20 notices per hour by
  default; the first notice after a limited hour says how many were dropped.
- Upstream model check: the consume log records the model sent upstream when a mapping renamed
  it, and flags a response whose declared model is neither the requested nor the mapped one
  (dated, regional and provider-path variants still match; Azure deployments are skipped). The
  request log shows a warning on those rows, and `GET /api/channel/cache_stats` counts them per
  channel.
- Channel price sync shows a review first: each channel model with its upstream price change
  (new, increase, decrease) and the user groups priced below the channel's cost ratio. Only the
  selected models are written.
- Failed requests are logged: when a request still fails after retries, a log entry records the
  status and message the client received, the channels tried and their status codes. The request
  log page switches between succeeded and failed requests, and the log details show the retry chain
  (the provider responses list and the fallback count) for both. The "Record failed requests"
  switch in the log settings turns it off.
- The scheduled channel test is set in the gateway settings (test mode and interval). The new
  "recovery" mode tests only auto-disabled channels and re-enables those that pass, so healthy
  channels spend no quota on tests.
- Session affinity on every channel type: requests of one client session stay on the channel that
  served the session, so the upstream prompt cache keeps hitting when several channels serve a
  model. Sessions are identified by the Claude Code session in `metadata.user_id`, a session
  header (`session_id`, as sent by Codex), the OpenAI `prompt_cache_key`, or the same system prompt
  and first message, scoped to the API key and model. It is on by default and works without Redis
  (an in-process map is used). The new "Session affinity" settings card turns it off, sets the
  binding lifetime (default one hour after the last request) and offers a strict mode that returns
  the error instead of moving a session whose channel fails. Each consume log records whether the
  request stayed on its bound channel (`session_affinity`), and `GET /api/channel/cache_stats`
  reports per-channel requests, prompt and cache read tokens and affinity counts.
- Editing a saved API key shows the models it can actually call, taking the group, the model limit
  and, for organization keys, the member allowlist into account (`GET /api/token/:id/models`,
  `GET /api/org/:id/token/:token_id/models`).
- Vercel AI Gateway channel type: OpenAI-compatible relay, model list with prices and metadata
  synced like OpenRouter, and balance lookup through `/v1/credits`.
- OpenRouter channels report the key's remaining amount as the channel balance and store a quota
  snapshot (limit, remaining, reset interval, daily / weekly / monthly usage).
- Channels have a low-balance alert threshold; the periodic balance refresh
  (`CHANNEL_UPDATE_FREQUENCY`) notifies once when the balance drops below it.

### Changed

- The channel form's model selector is rebuilt around model families: it fetches the upstream
  list when opened, lists families hottest first, and shows only the newest version of each
  family (older versions fold behind "N older"; a search shows every match). A filter panel
  narrows by selection status, models already configured in other channels, open or closed
  weights, vendor, capability (tools, reasoning, structured output, vision), output modality,
  maximum input / output price and minimum context; "Select results" adds every shown model.
  Rows show context length, site price, and badges for latest, preview, variants, aliases, open
  weights and upcoming retirement. On narrow screens the filters fold behind a button.
- The daily upstream model check (`MODEL_DRIFT_AUTO_CHECK`) is on by default.
- Automatic price syncs (startup, daily, channel save) write new models and price increases, and
  hold decreases for review; held changes and locked prices below the upstream price are sent as
  notifications. The "Automatic price sync" setting in the billing settings can write every change
  (the previous behavior) or new models only.
- Failover defaults: retries default to 2 (never more than the channels available), automatic
  re-enabling of auto-disabled channels defaults to on, and the scheduled channel test defaults to
  recovery mode every 10 minutes. Sites that saved their own values keep them;
  `CHANNEL_TEST_FREQUENCY` still tests every channel at its interval when set.
- API key model limits and organization member allowlists use the same matching rules:
  case-insensitive, `*` prefix wildcards, and aliases count as their canonical model. A key's model
  limit with no model selected now means no restriction, as the documentation already said;
  existing keys whose limit was on with an empty list (which rejected every request) are disabled
  instead of being opened up.
- "Members can view everyone's usage" is a plain organization switch: the site-wide default for it
  is removed and new organizations start with it off, so members only see their own usage until an
  owner or admin turns it on. Existing organizations keep their current behavior.
- Periodic and bulk balance refresh covers every channel type with a balance API instead of
  OpenAI only, and skips types without one; the single-channel refresh reports "not supported"
  for those types.
- Saving a channel no longer overwrites its balance, quota snapshot or model drift result with the
  form's stale values.
- Secrets are shown with one masking rule everywhere: 16 or more characters show the first four
  and last four (`sk-t****ghij`), 8 to 15 show only the last four, shorter values show nothing.
  This applies to channel keys (previously the last four only), API tokens, the OIDC provider
  client secret and the secret system settings (SMTP password, GitHub and LINUX DO client
  secrets, Turnstile secret key, WeChat server token, Cloudflare Worker key), which previously
  showed nothing once saved. Leaving a secret field blank keeps the stored value.
- The "Models" tab of the channel form is renamed "Served models" so it no longer reads like the
  Models page in the admin sidebar.

### Fixed

- Azure v1 channels accept `/v1/images/edits` and `/v1/images/variations` instead of answering
  501 `unsupported_api`.

### Security

- Upstream errors that reached clients without the usual masking are now masked: errors arriving
  after a stream started (both stream formats), Midjourney submit errors and image fetch errors,
  and Suno / Kling task errors could carry the channel's upstream address, proxy or key.
- Links to providers' public documentation sites (OpenAI, Anthropic, Google AI, AWS, Azure,
  OpenRouter, Mistral, DeepSeek, xAI, Groq) stay readable in error messages; channel addresses,
  proxies, IPs and keys are still masked.
- Removed a debug print that wrote every relay error body to standard output.
- The admin payment gateway endpoints no longer return gateway credentials (Epay key, Alipay
  private key, WeChat Pay APIv3 key and private key, Stripe secret key and webhook secret) in
  clear text. They are returned masked (first four and last four characters, shorter values show
  less), the edit form displays the mask next to an empty field, and leaving the field blank keeps
  the stored credential.

## [0.2.0] - 2026-10-18

### Added

- Channels have a remark field for admin notes, shown under the channel name. It is kept per
  channel and not shared across a tag.
- Channels can adjust which chat models are sent to the Responses API ("Chat-to-Responses
  models"): listed models are added to the built-in list, and a `-` prefix removes one.
- A multi-model test dialog tests several models of a channel at once, with an optional
  endpoint override. Image, video and deep research models are not selected by default.
- The multi-model test dialog can retry only the failed models and test a single row; a batch
  cannot start while single-row tests are running.
- The channel list API accepts `exact_model`, which matches a whole entry of a channel's model
  list. Batch model removal uses it, so searching `gemini-2.5-flash` no longer lists every
  `gemini-2.5-flash-*` channel and pushes the wanted ones past the first 100 results.
- Admin API responses can carry a stable `code` (snake_case) and optional `params` next to the
  unchanged English `message`; the 429 rate-limit body adds `params.retry_after` under `error`.
  The web console shows coded messages in the interface language (English, Simplified Chinese,
  Traditional Chinese, Japanese) and falls back to `message` for anything else. The first codes
  cover channel tests (the upstream reason stays as is), rate limiting and common channel and
  channel tag validation errors.
- The built-in default price table covers current models at their official list prices,
  including cache read and write prices and long-context tiers: OpenAI GPT-6 (Astra, 6.1 Sol, Sol,
  Luna), GPT-5.x and the Codex models, GPT-4.1, o-series; Anthropic Claude Fable 5 / 5.1, Opus,
  Sonnet and Haiku 4.x / 5.x; Gemini 4 Argon, 3.x and 2.5 (with audio input and image output
  prices); and DeepSeek Flash / V4 Pro. Model names that Antigravity maps to Claude or Gemini
  models use the target model's price. Codex, Claude Code, Gemini CLI, Antigravity, Bedrock and
  Vertex AI channels bill these models at the same prices, since prices are keyed by model name.
  New deployments no longer reject these models under the default `unpriced_model_policy=block`;
  existing deployments get the new entries on restart with `auto_price_updates` on, or through
  `Update Prices` with `/api/prices?type=default` as the price source.
- The model search box of the channel form suggests models as you type: the default models of
  the channel type and the price-table models of that type (Azure OpenAI channels use the OpenAI
  entries, Vertex AI Express the Gemini ones). Models already in the list are not suggested.
  Enter still adds the typed text, and "Edit as text" is unchanged.
- Codex channels can sign in with a device code, the default in the authorization dialog: the
  console shows a one-time code to enter at `auth.openai.com/codex/device` from any browser,
  so nothing has to reach `localhost:1455`. The first use needs device code login turned on in
  ChatGPT Settings → Security (a workspace admin turns it on for workspace accounts). New admin
  endpoints `POST /api/codex/oauth/device/start` and `POST /api/codex/oauth/device/poll`; the
  credentials are returned once, in the poll response, to the admin who started the login.

### Changed

- The provider dropdown of the channel form lists common providers first (OpenAI, Anthropic,
  Gemini, Azure OpenAI, Amazon Bedrock, Vertex AI, OpenRouter, DeepSeek, Codex, Claude Code and
  the custom channel), then a separator and every other provider sorted by name. Search covers
  both parts.
- Default models of new channels are the current ones: Anthropic, Bedrock and Claude Code list
  Claude 4.5 to 5.5 (retired Claude 1 to 3 models are gone); Gemini, Vertex AI and Vertex AI
  Express list Gemini 4 Argon and 3.x; DeepSeek, Moonshot (Kimi), Mistral, Zhipu, MiniMax and
  Cohere follow their current model lists; Codex, xAI and Azure Databricks get default models.
  Vertex AI also lists Claude 4.5 to 5.5. Groq no longer prefills its retired models; use **Fetch
  models**. Test models are cheap models of each list.
- The **Fetch models** button is shown only for channel types whose backend can list models;
  xAI and Ollama channels get it.
- A manual channel test no longer disables the channel when it fails; the scheduled test of
  all channels still does. Only tests of the channel's test model update its response time.
- **The channel check is rewritten.** It now checks one model per request at
  `GET /api/channel/check/:id?model=` (admin only); the dialog runs two models at a time, warns
  that each model sends up to five real requests to the upstream, and asks before starting. The
  checks are a random arithmetic challenge instead of "hi", reading digits from a locally
  rendered image, a JSON schema answer, two parallel tool calls and a relay hint. Each result is
  passed, warning, failed or not applicable, and only a failed basic check fails a model. Checks
  that a request conversion makes meaningless (response_format on channels converted to Claude,
  the invalid-role probe on Claude and Gemini conversions) are marked not applicable instead of
  failed, and upstream errors are grouped into categories such as authentication, quota or rate
  limit. Results are returned as stable codes and translated in the web console. The SSE endpoint
  `POST /api/sse/channel/check` is removed; scripts calling it must switch to the new endpoint.
- Codex channels now report Codex CLI version 0.159.0 (was 0.144.1) in the `version` header and
  the default User-Agent, since the upstream rejects new models for clients below a minimum
  version. A channel can set its own version with the new **Client version** plugin setting
  (`codex.client_version`); a value that is not a version such as `0.159.0` or
  `0.158.0-alpha.2` is logged and the default is used.
- **Codex channels no longer add the Codex CLI system prompt to requests without
  instructions.** Chat requests and `/v1/responses` requests that bring no `instructions` now
  get the one-line "You are a helpful assistant." instead of the 2025 coding-agent prompt of
  about 24,000 characters; instructions sent by the client are passed through unchanged. On a
  real Codex channel this bills about 5,000 fewer input tokens per request (a one-word chat
  prompt went from 5,017 to 22 input tokens). The output style changes with it: answers no
  longer follow the coding-agent conventions of that prompt, and tool calls start without a
  preamble sentence. Clients that relied on that behavior should send their own instructions.
- The default price of `gpt-4o` is the current official price ($2.50 / $10 per 1M tokens, cached
  input $1.25) instead of the 2024-05-13 snapshot price. Built-in defaults never overwrite
  existing price entries; update `gpt-4o` from `Update Prices` if needed.
- The ClaudeCode and Codex paste dialogs explain what to copy for each provider (the Codex
  error page's address, the Claude authentication code), check the pasted text as it is typed
  and submit a valid paste by itself. Pasting a callback from another login attempt, the
  authorization link itself, a provider error or an already used code now gives a specific,
  translated message.

### Fixed

- Sign-in, sign-up, password reset, email code and account security errors (wrong password,
  validation, invite code, verification code, expired session, unlinking the last sign-in
  method and similar) were shown in English whatever the interface language. These responses
  now carry a `code` and the web console shows them in the interface language; `message` is
  unchanged. Email labels on the sign-up and forgot-password pages are translated as well.
- Organization errors (members, invitations, roles, ownership transfer, budgets, quota transfer,
  top-ups and organization API keys) and admin user management errors (role limits, root user
  protection, shadow accounts, quota changes) are shown in the interface language as well. An
  unknown or malformed user ID in the admin user API now answers "User not found" or "invalid
  user ID" instead of the database or parser text.
- System settings, invite code, redemption code, API key, model catalog, order, payment method,
  invoice and third-party sign-in (GitHub, LINUX DO, Lark, WeChat, OIDC) messages are shown in
  the interface language as well. Redeeming an invalid, used or expired code keeps its reason
  instead of a generic failure, and the invoice month and log context prompts in the console are
  translated.
- Password length errors said "at most 20 characters" while the limit is 64; length messages
  now take their limits from the validation rules.
- When Turnstile was enabled and the session could not be saved after a successful check, the
  request still went on to the handler after the error response.
- Chat requests on Codex channels returned no tool calls, streaming or not: function call
  events were dropped and only the text was forwarded. They now return `tool_calls` with
  `finish_reason: "tool_calls"`, and stream reasoning text as well.
- Streamed chat answers converted from the Responses API end with `finish_reason:
  "tool_calls"` instead of `"stop"` when the model called a function, as non-streamed answers
  already did.
- `/v1/responses` on Codex channels accepts a string `input`, as the public Responses API does;
  the upstream rejected it with 400 "Input must be a list".
- The ClaudeCode and Codex code exchange checks that a pasted callback belongs to the login
  session, and no longer ends the session when the paste is wrong or the code is rejected, so
  the admin can paste again or reopen the same authorization link within its 10 minutes. Only
  the admin who started a login can finish it.
- Moderation requests without a `model` now default to `omni-moderation-latest`, since the
  `text-moderation` models were retired upstream. `omni-moderation` models are priced at 0 by
  default.
- Chat requests routed to the Responses API (o1-pro, o3-pro, deep-research and
  `codex-mini-latest`) now honor the channel model mapping, always return chat completions
  (also after a retry on a passthrough channel), and stream chat chunks on Codex channels.
- Channel tests for those models use the Responses API, matching real traffic.
- Adding keys while editing a channel tag no longer silently drops the new channels.
- The channel check applies the channel's model mapping and `#` suffix, uses the Responses API
  for Responses-only models like real traffic does, and masks upstream URLs and keys in errors.
  A provider panic during a check no longer crashes the server, and a closed dialog no longer
  leaves check goroutines blocked.
- The image check no longer needs a server address reachable from the internet, and the public
  `GET /api/image/:id` endpoint it relied on is removed: anyone could add access records to a
  running check through it without signing in.
- Chat requests sent to Claude channels (Anthropic, Claude Code, Bedrock and Vertex AI) now treat
  `developer` messages as system instructions instead of sending them as assistant messages.
  Several system messages are joined with line breaks, and messages with an unknown role are sent
  as user messages.
- Codex channels with a custom `user_agent` in their settings no longer drop the `version` and
  `originator` headers; the custom value now replaces only the User-Agent.
- Channel tag endpoints answered missing tags and invalid status or setting types with an error
  shape the web console could not read, so no message was shown; they now answer
  `{success: false, message, code}` like the other admin endpoints.
- Malformed or unknown channel ids now answer "invalid channel id" / "channel not found"
  instead of raw parser and database text.
- Binding LINUX DO while signed in with an access token no longer crashes the request, and an
  account already linked to LINUX DO must be unlinked before it can be linked to another one.
- AWS Bedrock sent `claude-haiku-5-5` by its bare model id, which AWS rejects because the model
  is only available through inference profiles. It now uses the `us.` profile in US and Canadian
  regions, `eu.` in EU regions and `global.` elsewhere.

### Security

- The Gemini CLI and Antigravity OAuth callback pages, which need no sign-in, put the
  `error_description` query parameter into the page unescaped, so a crafted link could run
  script on the console's origin. All text on these pages is now escaped, the pages send a
  nonce-based Content-Security-Policy, and they no longer use inline event handlers.
- These callback pages sent the OAuth credentials with `postMessage` to any opener window.
  They now send them only to the console origin that started the authorization (taken from
  the start request), and fall back to polling when that origin is unknown; the console only
  accepts messages from the popup it opened.
- The admin user list and user detail endpoints returned each user's `access_token`, so an
  admin could read the root user's token and act as root. Only `/api/user/self` returns it now.

## [0.1.1] - 2026-10-10

### Changed

- The home page hero figure is redrawn as symmetric line art: lines from members, apps and API
  keys gather into the Modeltaps mark and fan out to model providers, with dots on both ring
  crossings. The documentation site home page uses the same figure, and the figure's
  screen-reader description is updated in every language.
- The Hong Kong Chinese environment variable guide now uses Hong Kong wording for "account".

## [0.1.0] - 2026-10-09

### Added

- **First Modeltaps release line**: the repository starts fresh from the code of its private
  predecessor, without the earlier history. Upstream attribution is in NOTICE. It ships with the
  organization and multi-tenant layer, the shadcn web console, the model catalog and the native
  API console already in place. Deploy it as a new installation; no upgrade path from earlier
  builds is provided.
- Deployment guides now cover setting the server address after install and tightening data
  directory permissions.
- Channels have a new switch for the unified request/response model, available in the channel
  settings of the web console.
- Rate-limited requests (HTTP 429) now include `Retry-After` and `X-RateLimit-*` headers and a
  JSON error body with the wait time and request id; the web console shows this message.
- Global rate limits now count per signed-in user as well as per source IP, so users behind a
  shared NAT no longer block each other. Trusted IP/CIDR sources can be exempted with
  `GLOBAL_RATE_LIMIT_WHITELIST`, and local loopback requests are not counted.
- Channel auto-disable logs and notifications now name the model that triggered the disable.
- AWS Bedrock supports `claude-haiku-5-5`, with a default price.
- New AWS Bedrock channels are prefilled with the current Claude models (Sonnet 4.5 to Fable
  5.1) instead of only the Claude 2 and 3 models.

### Fixed

- Static assets no longer count toward the web rate limit, so pages no longer go blank after a
  few reloads.
- Requests sent through a channel proxy now reuse connections per proxy address instead of
  opening new ones each time.
- Gemini image models are detected more reliably; streaming image output is billed at the image
  output ratio, and Imagen requests are billed only for images actually returned, with a content
  policy error when every image is blocked.

### Security

- Changing your own password now requires the current password.
- Epay payment callbacks are verified more strictly: unexpected parameters are rejected,
  signatures are compared in constant time, the trade status is checked only after signature
  verification, and concurrent callbacks for the same order are serialized.
