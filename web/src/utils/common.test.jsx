import { beforeEach, describe, expect, it, vi } from 'vitest';

const get = vi.fn();
vi.mock('./api', () => ({ API: { get: (...args) => get(...args) } }));

const i18n = { language: 'zh_CN', t: (key) => key };
vi.mock('i18n/i18n', () => ({ default: i18n }));

const { getOIDCEndpoint } = await import('./common');

// 授权地址请求带上当前界面语言(ui_locales),让 IdP 登录页与本站语言一致;
// 语言未知时不带该参数,请求与改动前一致。

describe('getOIDCEndpoint', () => {
  beforeEach(() => {
    get.mockReset();
    get.mockResolvedValue({ data: { success: true, data: 'https://idp.example.com/auth' } });
    i18n.language = 'zh_CN';
  });

  it('按当前界面语言携带 ui_locales', async () => {
    i18n.language = 'zh_HK';
    await expect(getOIDCEndpoint('keycloak')).resolves.toBe('https://idp.example.com/auth');
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint/keycloak?ui_locales=zh-HK');
  });

  it('语言未知时不带 ui_locales', async () => {
    i18n.language = 'fr_FR';
    await getOIDCEndpoint('keycloak');
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint/keycloak');
  });

  it('无 slug 时走旧路由并同样带上语言', async () => {
    await getOIDCEndpoint();
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint?ui_locales=zh-CN');
  });
});
