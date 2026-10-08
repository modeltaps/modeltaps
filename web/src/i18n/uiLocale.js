import i18n from './i18n';

// 界面语言码 → OIDC ui_locales 的 BCP47 标签（OIDC Core 3.1.2.1）。
// 后端只放行这四个值，其它一律忽略，所以未知语言这里直接返回空串、由调用方省略该参数。
const BCP47_BY_LNG = {
  zh_CN: 'zh-CN',
  zh_HK: 'zh-HK',
  en_US: 'en',
  ja_JP: 'ja'
};

export function toBcp47(lng) {
  return BCP47_BY_LNG[lng] || '';
}

// 当前界面语言对应的 `?ui_locales=` 查询串（含 `?`）；语言未知时为空串，请求与改动前逐字节一致。
export function uiLocalesQuery() {
  const locale = toBcp47(i18n.language);
  return locale ? `?ui_locales=${encodeURIComponent(locale)}` : '';
}
