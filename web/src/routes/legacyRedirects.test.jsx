// @vitest-environment jsdom
import { createElement } from 'react';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { OrgContext } from 'contexts/OrgContext';
import { legacyRedirects } from './legacyRedirects';

// Every URL the console used before the current scheme still lands on its page.

afterEach(cleanup);

function Where() {
  const { pathname, search, hash } = useLocation();
  return createElement('output', null, `${pathname}${search}${hash}`);
}

const orgs = [{ id: 4, slug: 'acme', name: 'Acme', role: 'owner' }];

function landing(url, org = {}) {
  const router = createMemoryRouter([{ path: '/panel', children: [...legacyRedirects, { path: '*', element: createElement(Where) }] }], {
    initialEntries: [url]
  });
  const value = { orgEnabled: true, orgsLoaded: true, organizations: orgs, currentOrgId: null, ...org };
  render(createElement(OrgContext.Provider, { value }, createElement(RouterProvider, { router })));
  return screen.getByRole('status').textContent;
}

const cases = [
  ['/panel/log?token_name=k', '/panel/logs?token_name=k'],
  ['/panel/log?tab=task', '/panel/logs?tab=task'],
  ['/panel/task', '/panel/logs/tasks'],
  ['/panel/midjourney', '/panel/logs/midjourney'],
  ['/panel/token', '/panel/keys'],
  ['/panel/model_price', '/panel/models'],
  ['/panel/routing', '/panel/routing-policies'],
  ['/panel/api', '/panel/docs'],
  ['/panel/api/docs/image', '/panel/docs/image'],
  ['/panel/api/chat?model=x#playground', '/panel/playground/chat?model=x#playground'],
  ['/panel/ledger', '/panel/billing'],
  ['/panel/invoice', '/panel/billing/invoices'],
  ['/panel/topup', '/panel/billing?topup=1'],
  ['/panel/profile', '/panel/settings/profile'],
  ['/panel/profile?highlight=log-io', '/panel/settings/privacy?highlight=log-io'],
  ['/panel/settings/account/security', '/panel/settings/security'],
  ['/panel/settings/org/4/members', '/panel/settings/orgs/acme/members'],
  ['/panel/settings/org/99/members', '/panel/settings'],
  ['/panel/organization?invite=t1', '/panel/invitations?invite=t1'],
  ['/panel/analytics', '/panel/admin/stats'],
  ['/panel/multi_user_stats', '/panel/admin/stats/users'],
  ['/panel/system_info', '/panel/admin/system-logs'],
  ['/panel/channel', '/panel/admin/providers'],
  ['/panel/channel/12?tab=keys', '/panel/admin/providers/12?tab=keys'],
  ['/panel/allocation', '/panel/admin/providers'],
  ['/panel/pricing?tab=vendors', '/panel/admin/models?tab=vendors'],
  ['/panel/model_info', '/panel/admin/models'],
  ['/panel/model_ownedby', '/panel/admin/models/vendors'],
  ['/panel/routing_policy', '/panel/admin/routing'],
  ['/panel/user?keyword=bob', '/panel/admin/users?keyword=bob'],
  ['/panel/user_group', '/panel/admin/user-groups'],
  ['/panel/org_admin', '/panel/admin/orgs'],
  ['/panel/payment', '/panel/admin/orders'],
  ['/panel/redemption', '/panel/admin/redemption-codes'],
  ['/panel/invite_code', '/panel/admin/invite-codes'],
  ['/panel/setting', '/panel/admin/settings'],
  ['/panel/setting/gateway?highlight=x', '/panel/admin/settings/gateway?highlight=x'],
  ['/panel/telegram', '/panel/admin/settings/notify?highlight=telegram']
];

describe('legacy redirects', () => {
  it.each(cases)('%s → %s', (from, to) => {
    expect(landing(from)).toBe(to);
  });

  it('the old organization console opens the last organization the user worked in', () => {
    localStorage.setItem('active_org_id', '4');
    expect(landing('/panel/organization')).toBe('/panel/orgs/acme/overview');
    cleanup();
    localStorage.removeItem('active_org_id');
    expect(landing('/panel/organization')).toBe('/panel/invitations');
  });
});
