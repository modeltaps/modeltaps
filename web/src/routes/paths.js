// ==============================|| ROUTES — PATH BUILDERS ||============================== //
// The one place that knows the console's URL scheme. Everything else builds links through these
// helpers; a test keeps `'/panel/...'` literals out of the rest of the code.
//
// Rules (see the route table in routes/MainRoutes.jsx):
//   - Three scopes: the personal workbench (`/panel/<page>`), an organization's workbench
//     (`/panel/orgs/:orgSlug/<page>`, same pages) and platform admin (`/panel/admin/<page>`).
//     Settings are a hub of their own (`/panel/settings/...`) that never changes the scope.
//   - Segments are lowercase kebab-case nouns matching the English menu label, plural for
//     collections. The only verb is `new`.
//   - A page's tabs are path segments (`/panel/logs/tasks`, `/panel/admin/models/vendors`); the
//     default tab is the bare page. View state (search, filters, paging, an open drawer's tab)
//     and one-shot actions (`?topup=1`) stay in the query string.
//   - Old URLs keep working through routes/legacyRedirects.js.

export const PANEL = '/panel';
export const ADMIN = `${PANEL}/admin`;
export const SETTINGS = `${PANEL}/settings`;
export const ORGS = `${PANEL}/orgs`;

const ORG_PREFIX = /^\/panel\/orgs\/([^/?#]+)/;

const segmentPrefix = (pathname, prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`);
const withTab = (base, tab) => (tab ? `${base}/${tab}` : base);

// ---------------------------------------------------------------- scopes

export const orgBase = (orgSlug) => `${ORGS}/${encodeURIComponent(orgSlug)}`;

// A workbench page in a scope: personal (no slug) or an organization.
export const scoped = (page, orgSlug = null) => `${orgSlug ? orgBase(orgSlug) : PANEL}/${page}`;

// The organization slug of an org-scoped path, or null.
export function orgSlugOf(pathname = '') {
  const m = pathname.match(ORG_PREFIX);
  if (!m) return null;
  try {
    return decodeURIComponent(m[1]);
  } catch {
    return m[1];
  }
}

// The same page in the personal scope: `/panel/orgs/acme/usage` → `/panel/usage`.
export const unscope = (pathname = '') => pathname.replace(ORG_PREFIX, PANEL);

// The same page in another scope (org slug, or null for personal).
export const rescope = (pathname, orgSlug = null) => {
  const personal = unscope(pathname);
  return orgSlug ? personal.replace(/^\/panel/, orgBase(orgSlug)) : personal;
};

export const isAdminPath = (pathname = '') => segmentPrefix(pathname, ADMIN);
export const isSettingsPath = (pathname = '') => segmentPrefix(pathname, SETTINGS);
export const isOrgPath = (pathname = '') => ORG_PREFIX.test(pathname);

// ---------------------------------------------------------------- workbench (personal or org)

export const DEFAULT_MODALITY = 'chat';

export const workbench = {
  dashboard: (org) => scoped('dashboard', org),
  usage: (org) => scoped('usage', org),
  logs: (tab = null, org = null) => withTab(scoped('logs', org), tab),
  keys: (org) => scoped('keys', org),
  models: (org) => scoped('models', org),
  routingPolicies: (org) => scoped('routing-policies', org),
  playground: (modality = DEFAULT_MODALITY, org = null) => scoped(`playground/${modality}`, org),
  // The API overview without a modality, one modality's reference with it.
  docs: (modality = null, org = null) => withTab(scoped('docs', org), modality),
  billing: (tab = null, org = null) => withTab(scoped('billing', org), tab)
};

// Organization-only pages.
export const org = {
  overview: (orgSlug) => `${orgBase(orgSlug)}/overview`
};

// Personal-only pages.
export const invitations = () => `${PANEL}/invitations`;
// Answering an agent's device authorization (AGENT-7); agents send users to `/device`.
export const agentDevice = () => `${PANEL}/device`;
export const inviteLink = (token) => `${invitations()}?invite=${encodeURIComponent(token)}`;

// ---------------------------------------------------------------- settings hub

export const ORG_SETTINGS_PREFIX = `${SETTINGS}/orgs`;
export const settings = {
  root: () => SETTINGS,
  account: (section) => `${SETTINGS}/${section}`,
  org: (orgSlug, section) => `${ORG_SETTINGS_PREFIX}/${encodeURIComponent(orgSlug)}/${section}`
};

// ---------------------------------------------------------------- platform admin

export const admin = {
  root: () => ADMIN,
  stats: () => `${ADMIN}/stats`,
  statsByUser: () => `${ADMIN}/stats/users`,
  systemLogs: (tab = null) => withTab(`${ADMIN}/system-logs`, tab),
  alerts: (tab = null) => withTab(`${ADMIN}/alerts`, tab),
  // `id` is a provider id or 'new'.
  providers: (id = null) => withTab(`${ADMIN}/providers`, id == null ? null : String(id)),
  models: (tab = null) => withTab(`${ADMIN}/models`, tab),
  routing: () => `${ADMIN}/routing`,
  users: () => `${ADMIN}/users`,
  userGroups: () => `${ADMIN}/user-groups`,
  orgs: () => `${ADMIN}/orgs`,
  orders: () => `${ADMIN}/orders`,
  redemptionCodes: () => `${ADMIN}/redemption-codes`,
  inviteCodes: () => `${ADMIN}/invite-codes`,
  settings: (section = null) => withTab(`${ADMIN}/settings`, section)
};

// ---------------------------------------------------------------- public

export const publicPricing = () => '/pricing';
