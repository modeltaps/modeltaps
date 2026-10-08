import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { clearReturnPath, trackSettingsReturnPath } from 'utils/adminPlane';
import { silenceRouterLayoutEffectWarning } from 'views/auth/authPageTestUtils';
import SettingsSidebar from './SettingsSidebar';

// 底部用户菜单依赖主题 / redux / 浏览器全局,与「返回」落点无关,换成空壳。
vi.mock('./MoreMenu', () => ({ default: () => null }));

silenceRouterLayoutEffectWarning();

// 整页设置面的「返回」落点:MainLayout 在设置面外的每次渲染记录来源(trackSettingsReturnPath),
// 侧边栏渲染期读取。来源只存内存,所以刷新/直达(= 没记过来源)回落 /panel/dashboard。

const render = (path) =>
  renderToStaticMarkup(createElement(MemoryRouter, { initialEntries: [path] }, createElement(SettingsSidebar, {})));

// 侧边栏顶部只有一个「返回」链接(已无品牌行),取第一个 href 即可。
const backHref = (html) => html.match(/href="([^"]*)"/)?.[1] ?? '';

const SETTINGS_PATH = '/panel/settings/account/security';

describe('SettingsSidebar back link', () => {
  beforeEach(() => clearReturnPath('settings'));

  it('一次导航直达设置路径:首屏「返回」回到来源页', () => {
    trackSettingsReturnPath('/panel/log', false);
    expect(backHref(render(SETTINGS_PATH))).toBe('/panel/log');
  });

  it('刷新 / 直达设置路径:没有来源,「返回」回落用户面首页', () => {
    expect(backHref(render(SETTINGS_PATH))).toBe('/panel/dashboard');
  });

  it('设置面内切换 section 不改写来源', () => {
    trackSettingsReturnPath('/panel/log', false);
    trackSettingsReturnPath('/panel/settings/account/profile', true);
    trackSettingsReturnPath(SETTINGS_PATH, true);
    expect(backHref(render(SETTINGS_PATH))).toBe('/panel/log');
  });

  it('经 /panel/profile 这类会重定向进设置面的旧路径:「返回」不回环', () => {
    trackSettingsReturnPath('/panel/log', false);
    trackSettingsReturnPath('/panel/profile', false);
    expect(backHref(render(SETTINGS_PATH))).toBe('/panel/dashboard');
  });

  // 返回行写去向:可见文案是目标面名称,aria-label 写「返回 X」。
  it('来源是管理后台路径:返回行文案指向管理后台', () => {
    trackSettingsReturnPath('/panel/channel', false);
    const html = render(SETTINGS_PATH);
    expect(backHref(html)).toBe('/panel/channel');
    expect(html).toContain('aria-label="返回管理后台"');
    expect(html).toContain('<span class="truncate">管理后台</span>');
  });

  it('来源是工作台路径:返回行文案指向工作台', () => {
    trackSettingsReturnPath('/panel/log', false);
    const html = render(SETTINGS_PATH);
    expect(html).toContain('aria-label="返回工作台"');
    expect(html).toContain('<span class="truncate">工作台</span>');
  });
});
