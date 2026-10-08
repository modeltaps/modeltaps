import { beforeEach, describe, expect, it, vi } from 'vitest';

const get = vi.fn();
vi.mock('utils/api', () => ({ API: { get: (...args) => get(...args) } }));

const i18n = { language: 'zh_CN' };
vi.mock('i18n/i18n', () => ({ default: i18n }));

const { SIGNED_OUT_PATH, requestLogout, signedOutUrl } = await import('./logout');

// 登出接口在 data.redirect_url 下发 IdP 结束会话地址:有值时前端整页跳过去,空串 / 缺失 / 请求失败时
// 由调用方自己落 /signed-out。登出的用户体验不能被接口异常挡住,所以这里一律降级为空串。

describe('requestLogout', () => {
  beforeEach(() => {
    get.mockReset();
    i18n.language = 'zh_CN';
  });

  it('调 /api/user/logout 并返回 redirect_url', async () => {
    get.mockResolvedValue({ data: { success: true, data: { redirect_url: 'https://idp.example.com/end?x=1' } } });
    await expect(requestLogout()).resolves.toBe('https://idp.example.com/end?x=1');
    expect(get).toHaveBeenCalledWith('/api/user/logout?ui_locales=zh-CN');
  });

  // IdP 侧若出现退出确认页,语言应跟随本站界面语言;界面语言未知时不带该参数。
  it('按当前界面语言携带 ui_locales', async () => {
    get.mockResolvedValue({ data: { success: true, data: { redirect_url: '' } } });
    i18n.language = 'ja_JP';
    await requestLogout();
    expect(get).toHaveBeenCalledWith('/api/user/logout?ui_locales=ja');

    i18n.language = 'fr_FR';
    await requestLogout();
    expect(get).toHaveBeenCalledWith('/api/user/logout');
  });

  it('redirect_url 为空串(无 IdP 会话)时返回空串', async () => {
    get.mockResolvedValue({ data: { success: true, data: { redirect_url: '' } } });
    await expect(requestLogout()).resolves.toBe('');
  });

  it('旧后端不下发 data / redirect_url 时返回空串', async () => {
    get.mockResolvedValue({ data: { success: true } });
    await expect(requestLogout()).resolves.toBe('');
  });

  it('非字符串的 redirect_url 不透传', async () => {
    get.mockResolvedValue({ data: { data: { redirect_url: 123 } } });
    await expect(requestLogout()).resolves.toBe('');
  });

  it('请求失败也不抛错,返回空串让调用方落本站已登出页', async () => {
    get.mockRejectedValue(new Error('network'));
    await expect(requestLogout()).resolves.toBe('');
  });
});

describe('SIGNED_OUT_PATH', () => {
  it('与后端登记给 IdP 的 post_logout_redirect_uri 路径一致', () => {
    expect(SIGNED_OUT_PATH).toBe('/signed-out');
  });
});

// 退出后整页跳转的落地地址:带上路由 basename(config.basename 为 '/' 时不重复斜杠)。
describe('signedOutUrl', () => {
  it('basename 为 / 时就是 SIGNED_OUT_PATH', () => {
    expect(signedOutUrl()).toBe(SIGNED_OUT_PATH);
    expect(signedOutUrl()).toBe('/signed-out');
  });
});
