import { describe, expect, it } from 'vitest';

import { filterUserSections, filterAdminSections, findActiveItem, isAdminPath } from './nav-config';

// N3 侧栏结构回归:用户侧栏只剩「主组 + API 组」,账单进主组(组织态仅 Owner/Admin),
// 充值 / 设置无侧栏入口但仍要解析出标题。

const idsOf = (sections) => sections.map((s) => ({ id: s.id, items: s.items.map((i) => i.id) }));

describe('filterUserSections', () => {
  it('个人上下文:主组 6 项 + API 组 6 项(五个模态控制台 + 接口文档)', () => {
    expect(idsOf(filterUserSections({ orgActive: false }))).toEqual([
      { id: 'nav_sec_build', items: ['dashboard', 'token', 'model_price', 'usage', 'log', 'billing'] },
      { id: 'nav_sec_api', items: ['api_chat', 'api_image', 'api_audio', 'api_compare', 'api_video', 'api_docs'] }
    ]);
  });

  it('组织上下文 Member:无「账单」,主组多「组织」一项,API 组不变', () => {
    expect(idsOf(filterUserSections({ orgActive: true, orgRole: 'member' }))).toEqual([
      { id: 'nav_sec_build', items: ['dashboard', 'token', 'model_price', 'usage', 'log', 'organization'] },
      { id: 'nav_sec_api', items: ['api_chat', 'api_image', 'api_audio', 'api_compare', 'api_video', 'api_docs'] }
    ]);
  });

  it('组织上下文 Owner/Admin:比 Member 多「账单」(组织池充值)', () => {
    expect(idsOf(filterUserSections({ orgActive: true, orgRole: 'admin' }))).toEqual([
      { id: 'nav_sec_build', items: ['dashboard', 'token', 'model_price', 'usage', 'log', 'billing', 'organization'] },
      { id: 'nav_sec_api', items: ['api_chat', 'api_image', 'api_audio', 'api_compare', 'api_video', 'api_docs'] }
    ]);
  });

  it('站点关闭内置试用区:API 组不再有受门禁的条目', () => {
    const api = filterUserSections({ orgActive: false, chatEnabled: false }).find((s) => s.id === 'nav_sec_api');
    expect(api.items.map((i) => i.id)).toEqual(['api_chat', 'api_image', 'api_audio', 'api_compare', 'api_video', 'api_docs']);
  });

  it('不含任何管理侧条目', () => {
    const all = filterUserSections({ orgActive: true, orgRole: 'owner' }).flatMap((s) => s.items);
    expect(all.some((i) => i.isAdmin)).toBe(false);
  });

  it('没有「管理」组:不进 navSections,由 Sidebar 在导航末尾自行追加', () => {
    const sections = filterUserSections({ orgActive: false, isAdmin: true });
    expect(sections.some((s) => s.id === 'nav_sec_admin')).toBe(false);
    expect(sections.flatMap((s) => s.items).some((i) => i.enterAdmin)).toBe(false);
  });
});

describe('findActiveItem', () => {
  it('解析 API 控制台页', () => {
    expect(findActiveItem('/panel/api/image')?.id).toBe('api_image');
    expect(findActiveItem('/panel/api/chat')?.id).toBe('api_chat');
    expect(findActiveItem('/panel/api/compare')?.id).toBe('api_compare');
  });

  it('解析接口文档页(所有模态共用一个标题)', () => {
    expect(findActiveItem('/panel/api/docs/chat')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/api/docs/speech')?.id).toBe('api_docs');
  });

  it('Playground 旧路径只做重定向,不再解析标题', () => {
    expect(findActiveItem('/panel/playground')).toBeNull();
    expect(findActiveItem('/panel/playground/speech')).toBeNull();
  });

  it('解析无侧栏入口的账户类页面', () => {
    expect(findActiveItem('/panel/topup')?.id).toBe('topup');
    expect(findActiveItem('/panel/billing')?.id).toBe('billing');
    expect(findActiveItem('/panel/settings')?.id).toBe('settings');
    expect(findActiveItem('/panel/settings/account/profile')?.id).toBe('settings');
  });

  it('统一设置页不与管理后台系统设置混淆', () => {
    expect(findActiveItem('/panel/setting/site')?.id).toBe('setting_site');
    expect(isAdminPath('/panel/settings')).toBe(false);
  });

  it('个人设置/账户流水/月度账单旧路径只做重定向,不再解析标题', () => {
    expect(findActiveItem('/panel/profile')).toBeNull();
    expect(findActiveItem('/panel/ledger')).toBeNull();
    expect(findActiveItem('/panel/invoice')).toBeNull();
  });

  it('日志页仍解析为 log(Tab 走 query string)', () => {
    expect(findActiveItem('/panel/log')?.id).toBe('log');
  });

  it('未登记路径返回 null', () => {
    expect(findActiveItem('/panel/not-a-page')).toBeNull();
  });
});

// 管理侧栏:5 组(概览 / 用户与组织 / 网关 / 财务 / 系统设置),系统设置 7 个主题平铺在最后一组。
// Telegram、模型归属、模型详情不再有侧栏入口。
const SETTING_IDS = [
  'setting_site',
  'setting_auth',
  'setting_billing',
  'setting_relay',
  'setting_privacy',
  'setting_integrations',
  'setting_organization'
];

describe('admin plane 结构', () => {
  it('root 可见 5 组,系统设置是最后一组', () => {
    expect(idsOf(filterAdminSections({ isRoot: true }))).toEqual([
      { id: 'nav_sec_overview', items: ['analytics', 'multi_user_stats', 'systemInfo'] },
      { id: 'nav_sec_people', items: ['user', 'user_group', 'org_admin'] },
      { id: 'nav_sec_gateway', items: ['channel', 'pricing'] },
      { id: 'nav_sec_finance', items: ['payment', 'redemption'] },
      { id: 'nav_sec_system', items: SETTING_IDS }
    ]);
  });

  it('非 root 管理员少 3 个 root-only 项(含「组织策略」主题)', () => {
    const nonRootIds = filterAdminSections({ isRoot: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(nonRootIds).toHaveLength(14);
    expect(nonRootIds).not.toContain('org_admin');
    expect(nonRootIds).not.toContain('systemInfo');
    expect(nonRootIds).not.toContain('setting_organization');
    expect(nonRootIds).toContain('channel');
    expect(nonRootIds).toContain('setting_site');
  });

  it('侧栏无 Telegram / 模型归属 / 模型详情', () => {
    const allIds = filterAdminSections({ isRoot: true }).flatMap((s) => s.items.map((i) => i.id));
    expect(allIds).not.toContain('telegram');
    expect(allIds).not.toContain('model_ownedby');
    expect(allIds).not.toContain('model_info');
  });

  it('isAdminPath 识别管理面路径(含被重定向的旧 url)', () => {
    expect(isAdminPath('/panel/channel')).toBe(true);
    expect(isAdminPath('/panel/setting/site')).toBe(true);
    expect(isAdminPath('/panel/setting')).toBe(true);
    expect(isAdminPath('/panel/telegram')).toBe(true);
    expect(isAdminPath('/panel/model_ownedby')).toBe(true);
    expect(isAdminPath('/panel/model_info')).toBe(true);
    expect(isAdminPath('/panel/dashboard')).toBe(false);
  });
});
