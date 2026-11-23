import { beforeEach, describe, expect, it, vi } from 'vitest';

const get = vi.fn();
vi.mock('./api', () => ({ API: { get: (...args) => get(...args) } }));

const i18n = { language: 'zh_CN', t: (key) => key };
vi.mock('i18n/i18n', () => ({ default: i18n }));

const { getOIDCEndpoint, HourPriceFormatter, MinutePriceFormatter } = await import('./common');

// 授权地址请求带上当前界面语言(ui_locales),让 IdP 登录页与本站语言一致;
// 语言未知时不带该参数,请求与改动前一致。

describe('getOIDCEndpoint', () => {
  beforeEach(() => {
    get.mockReset();
    get.mockResolvedValue({ data: { success: true, data: 'https://idp.example.com/auth' } });
    i18n.language = 'zh_CN';
  });

  it('按当前界面语言携带 ui_locales', async () => {
    i18n.language = 'ja_JP';
    await expect(getOIDCEndpoint('keycloak')).resolves.toBe('https://idp.example.com/auth');
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint/keycloak?ui_locales=ja');
  });

  it('语言未知时不带 ui_locales', async () => {
    i18n.language = 'xx_XX';
    await getOIDCEndpoint('keycloak');
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint/keycloak');
  });

  it('无 slug 时走旧路由并同样带上语言', async () => {
    await getOIDCEndpoint();
    expect(get).toHaveBeenCalledWith('/api/oauth/endpoint?ui_locales=zh-CN');
  });
});

// Gateway per-second prices ×60 carry rounding noise; the display keeps 4 significant digits.
describe('MinutePriceFormatter', () => {
  it('rounds per-minute prices to 4 significant digits', () => {
    expect(MinutePriceFormatter(0.0035004 / 0.002)).toBe('$0.0035');
    expect(MinutePriceFormatter(0.0016668 / 0.002)).toBe('$0.001667');
    expect(MinutePriceFormatter(3)).toBe('$0.006');
    expect(MinutePriceFormatter(0)).toBe('Free');
  });

  it('gives the same price per hour of audio', () => {
    expect(HourPriceFormatter(0.0035004 / 0.002)).toBe('$0.21');
    expect(HourPriceFormatter(0.0016668 / 0.002)).toBe('$0.10');
    expect(HourPriceFormatter(3)).toBe('$0.36');
  });
});
