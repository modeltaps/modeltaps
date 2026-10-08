// ==============================|| ORG SCOPE — active organization + API path rewriting ||============================== //
// Module-level holder for the active organization id, consumed by the axios
// request interceptor in utils/api.js. When an organization is active,
// personal-scope endpoints are rewritten to their /api/org/:id/* equivalents
// (paths per router/org-router.go). When no organization is active the
// rewriting is a no-op, so personal context behaves exactly as before.

const STORAGE_KEY = 'active_org_id';

let activeOrgId = null;

export function getActiveOrgId() {
  return activeOrgId;
}

// Set (or clear with null) the active org; persisted to localStorage so the
// selection survives page refreshes.
export function setActiveOrgId(id) {
  activeOrgId = Number.isInteger(id) && id > 0 ? id : null;
  try {
    if (activeOrgId) {
      localStorage.setItem(STORAGE_KEY, String(activeOrgId));
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // ignore storage errors (e.g. storage disabled)
  }
}

export function readStoredOrgId() {
  try {
    const id = parseInt(localStorage.getItem(STORAGE_KEY), 10);
    return Number.isInteger(id) && id > 0 ? id : null;
  } catch {
    return null;
  }
}

// Endpoints that must never be rewritten even though they share a prefix
// with a rewrite rule (no org equivalent exists).
// - /api/token/playground: personal-only playground token
// - /api/token/admin: site-admin token management (org-router has no admin routes)
const SKIP_PREFIXES = ['/api/token/playground', '/api/token/admin'];

// Ordered rules (first match wins). `from` is the personal-scope prefix,
// `to` is the suffix appended after /api/org/:id.
const REWRITE_RULES = [
  { from: '/api/token', to: '/token' }, // tokens CRUD → /api/org/:id/token*
  { from: '/api/log/self', to: '/logs' }, // self logs (+/stat,/export) → /api/org/:id/logs*
  { from: '/api/user/topup', to: '/topup' }, // redemption topup → /api/org/:id/topup
  { from: '/api/user/order', to: '/order' } // payment orders (+/status) → /api/org/:id/order*
];

// True when `url` starts with `prefix` at a path-segment boundary.
function matchPrefix(url, prefix) {
  if (!url.startsWith(prefix)) return false;
  const next = url.charAt(prefix.length);
  return next === '' || next === '/' || next === '?';
}

// Rewrite a personal-scope URL into the active organization scope.
// Returns the URL unchanged when no org is active or no rule matches.
export function rewriteOrgUrl(url) {
  if (!activeOrgId || typeof url !== 'string') return url;
  if (SKIP_PREFIXES.some((p) => matchPrefix(url, p))) return url;
  for (const rule of REWRITE_RULES) {
    if (matchPrefix(url, rule.from)) {
      let rest = url.slice(rule.from.length);
      // Personal group roots use a trailing slash (e.g. /api/token/) while org
      // routes don't — normalize to avoid gin 307 redirects.
      if (rest === '/') rest = '';
      else if (rest.startsWith('/?')) rest = rest.slice(1);
      return `/api/org/${activeOrgId}${rule.to}${rest}`;
    }
  }
  return url;
}
