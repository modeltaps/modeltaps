// Old platform admin URLs (before `/panel/admin/...`). They redirect (routes/legacyRedirects.jsx);
// the chrome keeps the admin sidebar during that redirect so it does not flash the workbench.
export const LEGACY_ADMIN_PATHS = [
  'analytics',
  'multi_user_stats',
  'system_info',
  'channel',
  'allocation',
  'pricing',
  'model_info',
  'model_ownedby',
  'routing_policy',
  'user',
  'user_group',
  'org_admin',
  'payment',
  'redemption',
  'invite_code',
  'setting',
  'telegram'
].map((p) => `/panel/${p}`);
