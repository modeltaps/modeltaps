import { describe, expect, it } from 'vitest';

import {
  commandSections,
  settingsCommandItems,
  contextSwitchTarget,
  deriveContext,
  findActiveItem,
  firstAdminUrl,
  isAdminPath,
  isSettingsPath,
  navSectionsFor
} from './nav-config';

const idsOf = (sections) => sections.map((s) => ({ id: s.id, items: s.items.map((i) => i.id) }));

const MAIN = { id: 'main', items: ['dashboard', 'usage', 'log'] };
const DEVELOP = { id: 'develop', items: ['token', 'model_price', 'playground', 'api_docs'] };

describe('deriveContext', () => {
  it('admin pages are the admin context for admins only', () => {
    expect(deriveContext({ pathname: '/panel/admin/providers', isAdmin: true })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/admin/settings/site', isAdmin: true, orgId: 3 })).toBe('admin');
    // Old admin URLs keep the admin chrome while they redirect.
    expect(deriveContext({ pathname: '/panel/channel', isAdmin: true })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/admin/providers', isAdmin: false })).toBe('personal');
  });

  it('workbench pages take their scope from the URL; 设置 keeps the active organization', () => {
    expect(deriveContext({ pathname: '/panel/dashboard', isAdmin: true })).toBe('personal');
    expect(deriveContext({ pathname: '/panel/dashboard', orgId: 7 })).toBe('personal');
    expect(deriveContext({ pathname: '/panel/orgs/acme/dashboard' })).toBe('org');
    expect(deriveContext({ pathname: '/panel/settings/profile', isAdmin: true, orgId: 7 })).toBe('org');
    expect(deriveContext({ pathname: '/panel/settings/profile' })).toBe('personal');
  });
});

describe('navSectionsFor (workbench)', () => {
  it('personal: main group and 开发, no 组织 group', () => {
    expect(idsOf(navSectionsFor({ context: 'personal' }))).toEqual([MAIN, DEVELOP]);
  });

  it('org member: the 组织 group only has the org overview', () => {
    expect(idsOf(navSectionsFor({ context: 'org', orgRole: 'member', orgSlug: 'acme' }))).toEqual([
      MAIN,
      DEVELOP,
      { id: 'organization', items: ['organization'] }
    ]);
  });

  it('org owner/admin: members & budget and billing; organization settings are in 设置, not the menu', () => {
    for (const orgRole of ['owner', 'admin']) {
      expect(idsOf(navSectionsFor({ context: 'org', orgRole, orgSlug: 'acme' }))).toEqual([
        MAIN,
        DEVELOP,
        { id: 'organization', items: ['organization', 'org_members', 'org_models', 'billing'] }
      ]);
    }
  });

  it('路由策略 is a workbench item in every context while the site allows own policies', () => {
    const features = { routingPolicy: true };
    for (const [context, orgRole] of [
      ['personal', null],
      ['org', 'owner'],
      ['org', 'member']
    ]) {
      const develop = navSectionsFor({ context, orgRole, orgSlug: 'acme', features }).find((s) => s.id === 'develop');
      expect(develop.items.map((i) => i.id)).toContain('routing');
      expect(develop.items.find((i) => i.id === 'routing').href).toBe(
        context === 'org' ? '/panel/orgs/acme/routing-policies' : '/panel/routing-policies'
      );
    }
    const off = navSectionsFor({ context: 'personal' }).find((s) => s.id === 'develop');
    expect(off.items.map((i) => i.id)).not.toContain('routing');
    expect(findActiveItem('/panel/routing-policies')?.id).toBe('routing');
    expect(findActiveItem('/panel/orgs/acme/routing-policies')?.id).toBe('routing');
  });

  it('switching context stays on 路由策略 while it is available', () => {
    const features = { routingPolicy: true };
    expect(
      contextSwitchTarget({ pathname: '/panel/routing-policies', to: { kind: 'org', orgSlug: 'acme', role: 'member' }, features })
    ).toBe('/panel/orgs/acme/routing-policies');
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/routing-policies', to: { kind: 'personal' }, features })).toBe(
      '/panel/routing-policies'
    );
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/routing-policies', to: { kind: 'personal' } })).toBe('/panel/dashboard');
  });

  it('org urls carry the organization slug', () => {
    const sections = navSectionsFor({ context: 'org', orgRole: 'owner', orgSlug: 'acme' });
    const org = sections.find((s) => s.id === 'organization');
    expect(org.items.find((i) => i.id === 'organization').href).toBe('/panel/orgs/acme/overview');
    expect(org.items.find((i) => i.id === 'org_members').href).toBe('/panel/settings/orgs/acme/members');
    expect(org.items.find((i) => i.id === 'billing').href).toBe('/panel/orgs/acme/billing');
    expect(sections.find((s) => s.id === 'main').items.map((i) => i.href)).toEqual([
      '/panel/orgs/acme/dashboard',
      '/panel/orgs/acme/usage',
      '/panel/orgs/acme/logs'
    ]);
    const personal = navSectionsFor({ context: 'personal' }).find((s) => s.id === 'main');
    expect(personal.items.map((i) => i.href)).toEqual(['/panel/dashboard', '/panel/usage', '/panel/logs']);
  });

  it('workbench menus never contain admin pages', () => {
    const ids = navSectionsFor({ context: 'org', orgRole: 'owner', orgSlug: 'acme', isRoot: true }).flatMap((s) =>
      s.items.map((i) => i.id)
    );
    expect(ids).not.toContain('channel');
    expect(ids).not.toContain('setting_site');
  });
});

describe('navSectionsFor (platform admin)', () => {
  it('root sees every group', () => {
    expect(idsOf(navSectionsFor({ context: 'admin', isRoot: true }))).toEqual([
      { id: 'admin_main', items: ['analytics', 'systemInfo'] },
      { id: 'gateway', items: ['channel', 'pricing', 'routing_policy', 'setting_gateway'] },
      { id: 'people', items: ['user', 'user_group', 'org_admin', 'setting_auth', 'setting_orgpolicy'] },
      { id: 'finance', items: ['payment', 'redemption', 'invite_code', 'setting_billing', 'setting_payment'] },
      { id: 'system', items: ['setting_site', 'setting_entries', 'setting_privacy', 'setting_mail', 'setting_notify', 'setting_modified'] }
    ]);
  });

  it('non-root admins lose 系统日志, 组织 and 组织策略', () => {
    const ids = navSectionsFor({ context: 'admin', isRoot: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).not.toContain('systemInfo');
    expect(ids).not.toContain('org_admin');
    expect(ids).not.toContain('setting_orgpolicy');
    expect(ids).toContain('analytics');
  });

  it('platform admin lands on 统计 for every admin', () => {
    expect(firstAdminUrl({ isRoot: true })).toBe('/panel/admin/stats');
    expect(firstAdminUrl({ isRoot: false })).toBe('/panel/admin/stats');
  });
});

describe('findActiveItem', () => {
  it('every API console resolves to 演练场, docs to 接口文档', () => {
    for (const m of ['chat', 'image', 'speech', 'compare', 'video'])
      expect(findActiveItem(`/panel/playground/${m}`)?.id).toBe('playground');
    expect(findActiveItem('/panel/orgs/acme/playground/chat')?.id).toBe('playground');
    expect(findActiveItem('/panel/docs/chat')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/docs/speech')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/docs')?.id).toBe('api_docs');
  });

  it('per-user stats share the 统计 item', () => {
    expect(findActiveItem('/panel/admin/stats/users')?.id).toBe('analytics');
    expect(findActiveItem('/panel/admin/stats')?.id).toBe('analytics');
  });

  it('every platform settings group resolves to its own item', () => {
    expect(findActiveItem('/panel/admin/settings')?.id).toBe('setting_site');
    expect(findActiveItem('/panel/admin/settings/site')?.id).toBe('setting_site');
    expect(findActiveItem('/panel/admin/settings/gateway')?.id).toBe('setting_gateway');
    expect(findActiveItem('/panel/admin/settings/billing')?.id).toBe('setting_billing');
    expect(isAdminPath('/panel/admin/settings/relay')).toBe(true);
    // Old platform settings URLs keep the admin chrome while they redirect.
    expect(isAdminPath('/panel/setting/relay')).toBe(true);
    expect(findActiveItem('/panel/admin/invite-codes')?.id).toBe('invite_code');
  });

  it('settings pages: personal vs organization', () => {
    expect(findActiveItem('/panel/settings')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/security')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/orgs/acme/general')?.id).toBe('org_settings');
    expect(findActiveItem('/panel/settings/orgs/acme/members')?.id).toBe('org_members');
    // Not ".../models": the server treats paths ending in /models as a mistyped relay URL.
    expect(findActiveItem('/panel/settings/orgs/acme/model-access')?.id).toBe('org_models');
    expect(findActiveItem('/panel/settings/orgs/acme/danger')?.id).toBe('org_settings');
    // The old budget section redirects to the 常规 group of the organization settings.
    expect(findActiveItem('/panel/settings/orgs/acme/budget')?.id).toBe('org_settings');
  });

  it('redirect-only legacy paths do not resolve', () => {
    for (const p of ['/panel/profile', '/panel/ledger', '/panel/invoice', '/panel/topup', '/panel/token', '/panel/not-a-page']) {
      expect(findActiveItem(p)).toBeNull();
    }
  });

  it('a path only matches whole segments', () => {
    expect(findActiveItem('/panel/logout')).toBeNull();
    expect(findActiveItem('/panel/admin/providers/12')?.id).toBe('channel');
    expect(findActiveItem('/panel/orgs/acme')).toBeNull();
  });
});

describe('isAdminPath', () => {
  it('recognises admin pages and legacy admin redirects', () => {
    for (const p of [
      '/panel/admin',
      '/panel/admin/providers',
      '/panel/admin/settings/site',
      '/panel/channel',
      '/panel/setting/site',
      '/panel/telegram',
      '/panel/model_info',
      '/panel/multi_user_stats'
    ]) {
      expect(isAdminPath(p)).toBe(true);
    }
    for (const p of ['/panel/settings', '/panel/settings/profile', '/panel/dashboard', '/panel/billing', '/panel/administrator']) {
      expect(isAdminPath(p)).toBe(false);
    }
  });
});

describe('contextSwitchTarget', () => {
  it('entering platform admin lands on its first visible page', () => {
    expect(contextSwitchTarget({ pathname: '/panel/logs', to: { kind: 'admin' }, isRoot: false })).toBe('/panel/admin/stats');
    expect(contextSwitchTarget({ pathname: '/panel/logs', to: { kind: 'admin' }, isRoot: true })).toBe('/panel/admin/stats');
  });

  it('leaving platform admin goes to the dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/admin/providers', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/admin/settings/site', to: { kind: 'org', orgSlug: 'beta', role: 'owner' } })).toBe(
      '/panel/orgs/beta/dashboard'
    );
  });

  it('shared workbench pages stay, under the other scope prefix', () => {
    expect(contextSwitchTarget({ pathname: '/panel/logs/tasks', to: { kind: 'org', orgSlug: 'beta', role: 'member' } })).toBe(
      '/panel/orgs/beta/logs/tasks'
    );
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/usage', to: { kind: 'org', orgSlug: 'beta', role: 'member' } })).toBe(
      '/panel/orgs/beta/usage'
    );
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/playground/image', to: { kind: 'personal' } })).toBe(
      '/panel/playground/image'
    );
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/billing', to: { kind: 'personal' } })).toBe('/panel/billing');
  });

  it('设置 does not depend on the workbench and stays', () => {
    expect(contextSwitchTarget({ pathname: '/panel/settings/profile', to: { kind: 'personal' } })).toBe('/panel/settings/profile');
    expect(
      contextSwitchTarget({ pathname: '/panel/settings/orgs/acme/general', to: { kind: 'org', orgSlug: 'beta', role: 'admin' } })
    ).toBe('/panel/settings/orgs/acme/general');
  });

  it('pages the target context lacks fall back to its dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/billing', to: { kind: 'org', orgSlug: 'beta', role: 'member' } })).toBe(
      '/panel/orgs/beta/dashboard'
    );
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/overview', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/orgs/acme/overview', to: { kind: 'org', orgSlug: 'beta', role: 'member' } })).toBe(
      '/panel/orgs/beta/overview'
    );
    expect(contextSwitchTarget({ pathname: '/panel/invitations', to: { kind: 'org', orgSlug: 'beta', role: 'member' } })).toBe(
      '/panel/orgs/beta/dashboard'
    );
  });
});

describe('commandSections', () => {
  it('non-admins get no platform admin entries', () => {
    const ids = commandSections({ isAdmin: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).toContain('personal_settings');
    expect(ids).not.toContain('org_settings');
    expect(ids).not.toContain('channel');
  });

  it('both parts of 设置: organization settings for owners and admins of the active organization', () => {
    const items = (orgRole) => commandSections({ orgSlug: 'acme', orgRole }).find((s) => s.id === 'account').items;
    const listed = (orgRole) => items(orgRole).filter((i) => !i.searchOnly);
    expect(listed('owner').map((i) => [i.id, i.href])).toEqual([
      ['org_settings', '/panel/settings/orgs/acme/general'],
      ['personal_settings', '/panel/settings']
    ]);
    expect(listed('member').map((i) => i.id)).toEqual(['personal_settings']);
  });

  it('every item of the 设置 tree is searchable (shown while typing)', () => {
    const typed = (orgSlug, orgRole) =>
      commandSections({ orgSlug, orgRole })
        .find((s) => s.id === 'account')
        .items.filter((i) => i.searchOnly)
        .map((i) => [i.id, i.href, i.prefixKey]);
    expect(typed('acme', 'admin')).toEqual([
      ['settings_org_general', '/panel/settings/orgs/acme/general', 'settingsUi.sections.org'],
      ['settings_org_members', '/panel/settings/orgs/acme/members', 'settingsUi.sections.org'],
      ['settings_org_model-access', '/panel/settings/orgs/acme/model-access', 'settingsUi.sections.org'],
      ['settings_org_danger', '/panel/settings/orgs/acme/danger', 'settingsUi.sections.org'],
      ['settings_personal_profile', '/panel/settings/profile', 'settingsUi.sections.personal'],
      ['settings_personal_security', '/panel/settings/security', 'settingsUi.sections.personal'],
      ['settings_personal_privacy', '/panel/settings/privacy', 'settingsUi.sections.personal'],
      ['settings_personal_preferences', '/panel/settings/preferences', 'settingsUi.sections.personal']
    ]);
    expect(typed('acme', 'member').map(([id]) => id)).not.toContain('settings_org_general');
    expect(typed(null, null).map(([id]) => id)).toHaveLength(4);
  });

  it('设置 pages are recognised by path', () => {
    for (const p of ['/panel/settings', '/panel/settings/profile', '/panel/settings/orgs/acme/members'])
      expect(isSettingsPath(p)).toBe(true);
    for (const p of ['/panel/admin/settings', '/panel/setting/site', '/panel/settingsx', '/panel/dashboard'])
      expect(isSettingsPath(p)).toBe(false);
  });

  it('admins can reach every admin page they may see', () => {
    const admin = commandSections({ isAdmin: true, isRoot: false }).find((s) => s.admin);
    expect(admin.items.map((i) => i.id)).toEqual([
      'admin_home',
      'analytics',
      'channel',
      'pricing',
      'routing_policy',
      'setting_gateway',
      'user',
      'user_group',
      'setting_auth',
      'payment',
      'redemption',
      'invite_code',
      'setting_billing',
      'setting_payment',
      'setting_site',
      'setting_entries',
      'setting_privacy',
      'setting_mail',
      'setting_notify',
      'setting_modified'
    ]);
    expect(admin.items[0].href).toBe('/panel/admin/stats');
  });

  it('platform settings cards only show while typing', () => {
    const items = commandSections({ isAdmin: true, isRoot: false }).find((s) => s.id === 'platform_settings').items;
    expect(items.every((i) => i.searchOnly)).toBe(true);
    const cards = settingsCommandItems({ isRoot: false });
    expect(cards.find((i) => i.id === 'setting_card_smtp').href).toBe('/panel/admin/settings/mail?highlight=smtp');
    expect(cards.some((i) => i.id === 'setting_card_org-policy')).toBe(false);
    expect(settingsCommandItems({ isRoot: true }).some((i) => i.id === 'setting_card_org-policy')).toBe(true);
  });
});
