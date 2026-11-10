import { describe, expect, it } from 'vitest';

import {
  commandSections,
  settingsCommandItems,
  contextSwitchTarget,
  deriveContext,
  findActiveItem,
  firstAdminUrl,
  isAdminPath,
  navSectionsFor
} from './nav-config';

const idsOf = (sections) => sections.map((s) => ({ id: s.id, items: s.items.map((i) => i.id) }));

const MAIN = { id: 'main', items: ['dashboard', 'usage', 'log'] };
const DEVELOP = { id: 'develop', items: ['token', 'model_price', 'playground', 'api_docs'] };

describe('deriveContext', () => {
  it('admin pages are the admin context for admins only', () => {
    expect(deriveContext({ pathname: '/panel/channel', isAdmin: true })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/setting/site', isAdmin: true, orgId: 3 })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/channel', isAdmin: false })).toBe('personal');
  });

  it('workbench pages follow the org context', () => {
    expect(deriveContext({ pathname: '/panel/dashboard', isAdmin: true })).toBe('personal');
    expect(deriveContext({ pathname: '/panel/dashboard', orgId: 7 })).toBe('org');
    expect(deriveContext({ pathname: '/panel/settings/account/profile', isAdmin: true, orgId: 7 })).toBe('org');
  });
});

describe('navSectionsFor (workbench)', () => {
  it('personal: main group and 开发, no 组织 group', () => {
    expect(idsOf(navSectionsFor({ context: 'personal' }))).toEqual([MAIN, DEVELOP]);
  });

  it('org member: the 组织 group only has the org overview', () => {
    expect(idsOf(navSectionsFor({ context: 'org', orgRole: 'member', orgId: 7 }))).toEqual([
      MAIN,
      DEVELOP,
      { id: 'organization', items: ['organization'] }
    ]);
  });

  it('org owner/admin: members & budget, billing and org settings', () => {
    for (const orgRole of ['owner', 'admin']) {
      expect(idsOf(navSectionsFor({ context: 'org', orgRole, orgId: 7 }))).toEqual([
        MAIN,
        DEVELOP,
        { id: 'organization', items: ['organization', 'org_members', 'billing', 'org_settings'] }
      ]);
    }
  });

  it('org-scoped urls carry the active organization', () => {
    const org = navSectionsFor({ context: 'org', orgRole: 'owner', orgId: 7 }).find((s) => s.id === 'organization');
    expect(org.items.find((i) => i.id === 'org_members').href).toBe('/panel/settings/org/7/members');
    expect(org.items.find((i) => i.id === 'org_settings').href).toBe('/panel/settings/org/7/general');
  });

  it('workbench menus never contain admin pages', () => {
    const ids = navSectionsFor({ context: 'org', orgRole: 'owner', orgId: 1, isRoot: true }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).not.toContain('channel');
    expect(ids).not.toContain('platform_settings');
  });
});

describe('navSectionsFor (platform admin)', () => {
  it('root sees every group', () => {
    expect(idsOf(navSectionsFor({ context: 'admin', isRoot: true }))).toEqual([
      { id: 'admin_main', items: ['systemInfo', 'analytics'] },
      { id: 'gateway', items: ['channel', 'pricing', 'routing_policy'] },
      { id: 'people', items: ['user', 'user_group', 'org_admin'] },
      { id: 'finance', items: ['payment', 'redemption', 'invite_code'] },
      { id: 'admin_settings', items: ['platform_settings'] }
    ]);
  });

  it('non-root admins lose 运行状况 and 组织', () => {
    const ids = navSectionsFor({ context: 'admin', isRoot: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).not.toContain('systemInfo');
    expect(ids).not.toContain('org_admin');
    expect(ids).toContain('analytics');
  });

  it('first admin page depends on root', () => {
    expect(firstAdminUrl({ isRoot: true })).toBe('/panel/system_info');
    expect(firstAdminUrl({ isRoot: false })).toBe('/panel/analytics');
  });
});

describe('findActiveItem', () => {
  it('every API console resolves to 演练场, docs to 文档', () => {
    for (const m of ['chat', 'image', 'speech', 'compare', 'video']) expect(findActiveItem(`/panel/api/${m}`)?.id).toBe('playground');
    expect(findActiveItem('/panel/api/docs/chat')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/api/docs/speech')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/api')).toBeNull();
  });

  it('per-user stats share the 统计 item', () => {
    expect(findActiveItem('/panel/multi_user_stats')?.id).toBe('analytics');
    expect(findActiveItem('/panel/analytics')?.id).toBe('analytics');
  });

  it('platform settings groups resolve to one item', () => {
    expect(findActiveItem('/panel/setting')?.id).toBe('platform_settings');
    expect(findActiveItem('/panel/setting/gateway')?.id).toBe('platform_settings');
    expect(findActiveItem('/panel/invite_code')?.id).toBe('invite_code');
  });

  it('settings pages: personal vs organization', () => {
    expect(findActiveItem('/panel/settings')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/account/security')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/org/3/general')?.id).toBe('org_settings');
    expect(findActiveItem('/panel/settings/org/3/members')?.id).toBe('org_members');
    expect(findActiveItem('/panel/settings/org/3/danger')?.id).toBe('org_settings');
    // The old budget section redirects to the 常规 group of the organization settings.
    expect(findActiveItem('/panel/settings/org/3/budget')?.id).toBe('org_settings');
  });

  it('redirect-only legacy paths do not resolve', () => {
    for (const p of ['/panel/playground', '/panel/profile', '/panel/ledger', '/panel/invoice', '/panel/topup', '/panel/not-a-page']) {
      expect(findActiveItem(p)).toBeNull();
    }
  });

  it('a path only matches whole segments', () => {
    expect(findActiveItem('/panel/logout')).toBeNull();
    expect(findActiveItem('/panel/channel/12')?.id).toBe('channel');
  });
});

describe('isAdminPath', () => {
  it('recognises admin pages and legacy admin redirects', () => {
    for (const p of [
      '/panel/channel',
      '/panel/setting',
      '/panel/setting/site',
      '/panel/telegram',
      '/panel/model_info',
      '/panel/multi_user_stats'
    ]) {
      expect(isAdminPath(p)).toBe(true);
    }
    for (const p of ['/panel/settings', '/panel/settings/account/profile', '/panel/dashboard', '/panel/billing']) {
      expect(isAdminPath(p)).toBe(false);
    }
  });
});

describe('contextSwitchTarget', () => {
  it('entering platform admin lands on its first visible page', () => {
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'admin' }, isRoot: false })).toBe('/panel/analytics');
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'admin' }, isRoot: true })).toBe('/panel/system_info');
  });

  it('leaving platform admin goes to the dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/channel', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/setting/site', to: { kind: 'org', orgId: 2, role: 'owner' } })).toBe('/panel/dashboard');
  });

  it('shared workbench pages stay', () => {
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'org', orgId: 2, role: 'member' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/api/image', to: { kind: 'personal' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/settings/account/profile', to: { kind: 'personal' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/billing', to: { kind: 'personal' } })).toBeNull();
  });

  it('org admin pages follow the new organization or fall back to the dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/budget', to: { kind: 'org', orgId: 2, role: 'admin' } })).toBe(
      '/panel/settings/org/2/budget'
    );
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/general', to: { kind: 'org', orgId: 2, role: 'member' } })).toBe(
      '/panel/dashboard'
    );
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/general', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/billing', to: { kind: 'org', orgId: 2, role: 'member' } })).toBe('/panel/dashboard');
  });
});

describe('commandSections', () => {
  it('non-admins get no platform admin entries', () => {
    const ids = commandSections({ isAdmin: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).toContain('personal_settings');
    expect(ids).not.toContain('channel');
  });

  it('admins can reach every admin page they may see', () => {
    const admin = commandSections({ isAdmin: true, isRoot: false }).find((s) => s.admin);
    expect(admin.items.map((i) => i.id)).toEqual([
      'admin_home',
      'analytics',
      'channel',
      'pricing',
      'routing_policy',
      'user',
      'user_group',
      'payment',
      'redemption',
      'invite_code',
      'platform_settings'
    ]);
    expect(admin.items[0].href).toBe('/panel/analytics');
  });

  it('platform settings groups are listed; cards only show while typing', () => {
    const groupIds = (isRoot) =>
      commandSections({ isAdmin: true, isRoot })
        .find((s) => s.id === 'platform_settings')
        .items.filter((i) => !i.searchOnly)
        .map((i) => i.href);
    expect(groupIds(false)).toEqual(
      ['site', 'auth', 'billing', 'payment', 'gateway', 'privacy', 'notify'].map((g) => `/panel/setting/${g}`)
    );
    expect(groupIds(true)).toContain('/panel/setting/orgpolicy');
    const cards = settingsCommandItems({ isRoot: false }).filter((i) => i.searchOnly);
    expect(cards.find((i) => i.id === 'setting_card_smtp').href).toBe('/panel/setting/notify?highlight=smtp');
    expect(cards.some((i) => i.id === 'setting_card_org-policy')).toBe(false);
  });
});
