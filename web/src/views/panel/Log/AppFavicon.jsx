import PropTypes from 'prop-types';
import { AppWindow } from 'lucide-react';

import BrandIcon from '@/components/brand/BrandIcon';
import { cn } from '@/lib/utils';

// ==============================|| APP FAVICON (SHARED) ||============================== //
// App 归因图标。日志表「应用」列与筛选下拉共用一份。
// 数据现状:多数客户端只带 X-Title(应用名)无 Referer → 日志无 app_domain。故按应用名内置
// 常见应用官网映射兜底,使不同应用呈现各自官方图标而非清一色回退。
// 图标源全部走本站接口(不直连任何第三方地址,离线可用):L1 内置品牌图标(应用名/域名命中
// manifest) → L2 后端缓存的站点 favicon(/api/brand-icon/domain/{domain}) → L3 通用 AppWindow 图标。

// 应用名规范化后精确匹配(含别名),不做子串模糊匹配以防误配。
const KNOWN_APP_DOMAINS = {
  cursor: 'cursor.com',
  zed: 'zed.dev',
  'zed editor': 'zed.dev',
  cline: 'cline.bot',
  'claude code': 'claude.ai',
  'roo code': 'roocode.com',
  'kilo code': 'kilocode.ai',
  continue: 'continue.dev',
  aider: 'aider.chat',
  windsurf: 'windsurf.com',
  'open webui': 'openwebui.com',
  lobechat: 'lobehub.com',
  'lobe chat': 'lobehub.com',
  librechat: 'librechat.ai',
  'cherry studio': 'cherry-ai.com',
  chatbox: 'chatboxai.app',
  'github copilot': 'github.com',
  sillytavern: 'sillytavern.app',
  dify: 'dify.ai',
  n8n: 'n8n.io',
  chatgpt: 'chatgpt.com',
  obsidian: 'obsidian.md'
};

// lowercase + trim + 折叠内部连续空白。
function normalizeAppName(name) {
  return String(name || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, ' ');
}

// domain 优先(来自日志/聚合的真实 Referer 域名);无 domain 时按应用名查内置映射。
function resolveDomain(name, domain) {
  if (domain) return domain;
  return KNOWN_APP_DOMAINS[normalizeAppName(name)] || '';
}

export default function AppFavicon({ name, domain, className }) {
  const appKey = normalizeAppName(name);
  const resolved = resolveDomain(name, domain);
  const sizeClass = cn('size-4', className);
  return (
    <BrandIcon
      key={`${appKey}|${resolved}`}
      brandKey={appKey}
      domain={resolved}
      className={sizeClass}
      fallback={<AppWindow className={cn('shrink-0 text-muted-foreground', sizeClass)} />}
    />
  );
}
AppFavicon.propTypes = { name: PropTypes.string, domain: PropTypes.string, className: PropTypes.string };

// appFaviconNode 供 .js 模块(如 logFilterFields.js,不能写 JSX)构造图标 ReactNode 使用:
// 返回 <AppFavicon name domain /> 元素,交由渲染方(ValuePanel EnumPanel)挂载。
export function appFaviconNode(name, domain) {
  return <AppFavicon name={name} domain={domain} />;
}
