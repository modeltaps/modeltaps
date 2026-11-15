import { useSelector } from 'react-redux';

import { useOrg } from 'contexts/OrgContext';
import { org, workbench } from 'routes/paths';

// ==============================|| PLAYGROUND — RUN GATE ||============================== //
// 门禁只卡「运行」这一步：组织上下文(试用区只覆盖个人 API Key)或站点关闭内置试用区时，
// 页面照常渲染(模态栏 / 设置栏 / 示例都在)，各模态把 gated 用于运行按钮置灰 + 一句说明 +
// 去向链接。本文件属 shared 层，禁止 import 任何 React 组件。

export const GATE_ORG = 'org';
export const GATE_CHAT_DISABLED = 'chatDisabled';

// 纯函数形式，便于单测三种输入；hook 只负责把上下文接进来。
export function resolveConsoleGate({ orgActive = false, orgSlug = null, chatEnabled = true } = {}) {
  if (orgActive) return { gated: true, reason: GATE_ORG, actionHref: orgSlug ? org.overview(orgSlug) : workbench.dashboard() };
  if (!chatEnabled) return { gated: true, reason: GATE_CHAT_DISABLED, actionHref: workbench.docs('chat') };
  return { gated: false, reason: null, actionHref: null };
}

export default function useConsoleGate() {
  const { currentOrgId, currentOrg } = useOrg();
  const siteInfo = useSelector((state) => state.siteInfo);

  return resolveConsoleGate({
    orgActive: Boolean(currentOrgId),
    orgSlug: currentOrg?.slug ?? null,
    chatEnabled: siteInfo?.builtin_chat_enabled !== false
  });
}
