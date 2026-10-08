// ==============================|| ORG SCOPE — rewrite rule assertions ||============================== //

import { describe, it, expect, beforeEach } from 'vitest';
import { setActiveOrgId, rewriteOrgUrl } from './orgScope.js';

// [input, expected] — expected === input means "must NOT be rewritten".
const cases = [
  // tokens CRUD → /api/org/:id/token*
  ['/api/token/', '/api/org/5/token'],
  ['/api/token/?page=1', '/api/org/5/token?page=1'],
  ['/api/token/123', '/api/org/5/token/123'],
  // skip-list: no org equivalent exists
  ['/api/token/playground', '/api/token/playground'],
  ['/api/token/admin', '/api/token/admin'], // site-admin token list (Token/index.jsx admin mode)
  ['/api/token/admin?page=1', '/api/token/admin?page=1'],
  ['/api/token/admin/search', '/api/token/admin/search'], // site-admin token search
  ['/api/token/admin/search?keyword=x', '/api/token/admin/search?keyword=x'],
  // self logs → /api/org/:id/logs*
  ['/api/log/self', '/api/org/5/logs'],
  ['/api/log/self/stat', '/api/org/5/logs/stat'],
  ['/api/log/self/export', '/api/org/5/logs/export'],
  // topup / orders
  ['/api/user/topup', '/api/org/5/topup'],
  ['/api/user/order', '/api/org/5/order'],
  ['/api/user/order/status', '/api/org/5/order/status'],
  // untouched personal/global endpoints
  ['/api/user/self', '/api/user/self'],
  ['/api/status', '/api/status'],
  ['/api/org/', '/api/org/'],
  // T9 org-management endpoints are already org-scoped — must pass through
  ['/api/org/invitations', '/api/org/invitations'], // my pending invitations (personal view)
  ['/api/org/invitations/accept', '/api/org/invitations/accept'],
  ['/api/org/5/members', '/api/org/5/members'],
  ['/api/org/5/audit_logs?page=1', '/api/org/5/audit_logs?page=1'],
  ['/api/available_model', '/api/available_model'],
  // T10 adapted-page endpoints — must never be rewritten
  ['/api/user/payment', '/api/user/payment'], // global payment methods (TopupCard)
  ['/api/user/dashboard', '/api/user/dashboard'], // personal dashboard (org view uses /api/org/:id/analytics)
  ['/api/org/5/analytics', '/api/org/5/analytics'], // explicit org path (Dashboard OrgUsagePanel)
  ['/api/organization/', '/api/organization/'], // site-admin org management (OrgAdmin)
  ['/api/option/', '/api/option/'], // site options (OrgAdmin global settings)
  // prefix must match at a path-segment boundary
  ['/api/tokenx', '/api/tokenx']
];

describe('rewriteOrgUrl (active org = 5)', () => {
  beforeEach(() => {
    setActiveOrgId(5);
  });

  it.each(cases)('%s -> %s', (input, expected) => {
    expect(rewriteOrgUrl(input)).toBe(expected);
  });
});

describe('rewriteOrgUrl (no active org)', () => {
  it('is a no-op when no org is active', () => {
    setActiveOrgId(null);
    expect(rewriteOrgUrl('/api/token/')).toBe('/api/token/');
  });
});
