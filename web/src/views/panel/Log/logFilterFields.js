import { FINISH_REASONS, RELAY_MODES } from './logHelpers';
import { appFaviconNode } from './AppFavicon';

// ==============================|| LOG — FILTER FIELD DEFINITIONS ||============================== //
// 驱动可复用 FilterBar 的字段定义(App 专属配置,非组件本身)。按上下文裁剪:
//   通用(requests):token_name / model_name / finish_reason / min_quota / min_tokens
//   组织上下文:member_id(选项来自成员列表)
//   平台管理员(非组织):channel_id / username / source_ip
// enum 字段的 paramInclude/paramExclude 与后端 model.LogsListParams/OrgLogsListParams 的
// form tag 一一对应(复数 = IN,exclude_ = NOT IN);单值旧字段仍由 Ledger 使用,互不影响。

const enumField = (key, labelKey, extra = {}) => ({
  key,
  labelKey,
  type: 'enum',
  supportsExclude: true,
  paramInclude: `${key}s`,
  paramExclude: `exclude_${key}`,
  ...extra
});

// token_name / model_name 预置选项来自真实数据源(API Key 名 / 可用模型);仍保留 freeText,
// 预置项之外可打字添加自定义值(值域可枚举者勾选,不可枚举者打字)。
const tokenField = (tokenOptions) =>
  enumField('token_name', 'tableToolBar.tokenName', {
    freeText: true,
    options: (tokenOptions || []).map((n) => ({ value: String(n), label: String(n) }))
  });

const modelField = (modelOptions) =>
  enumField('model_name', 'tableToolBar.modelName', {
    freeText: true,
    options: (modelOptions || []).map((m) => ({ value: String(m), label: String(m) }))
  });

// app_name 归因维度:预置选项来自用量分析 group_by=app 聚合(每项 { name, domain });仍保留 freeText,
// 平台管理员上下文无对应聚合接口时选项为空,可直接打字。key=app_name 使 IN/exclude 参数键与
// 后端 LogsListParams/OrgLogsListParams 的 app_names/exclude_app_name 对齐。option.icon 复用与日志表
// 同一份 AppFavicon(domain→favicon,无 domain 回退通用图标),由 ValuePanel 渲染。
const appField = (appOptions) =>
  enumField('app_name', 'logPage.appLabel', {
    freeText: true,
    options: (appOptions || []).map((a) => ({
      value: String(a.name),
      label: String(a.name),
      icon: appFaviconNode(a.name, a.domain)
    }))
  });

// 请求 ID 精确过滤(标量 text,后端 = 匹配):request_id 为本站追踪 ID,用户侧可用,
// 仅 admin/self 路径绑定(OrgLogsListParams 无此参数,组织上下文不展示);
// upstream_request_id 为上游厂商 ID,管理员维度(口径同 channel_id)。
const requestIdField = {
  key: 'request_id',
  labelKey: 'logPage.requestSection.requestId',
  type: 'text',
  opLabelKey: 'filterBar.opIsOne'
};

const upstreamRequestIdField = {
  key: 'upstream_request_id',
  labelKey: 'logPage.requestSection.upstreamRequestId',
  type: 'text',
  opLabelKey: 'filterBar.opIsOne'
};

// 与选项数据无关的通用字段(finish_reason 有静态选项;min_quota/min_tokens 为数值)。
const staticCommonFields = [
  {
    key: 'finish_reason',
    labelKey: 'logPage.finishReason.filterLabel',
    type: 'enum',
    supportsExclude: true,
    options: FINISH_REASONS,
    paramInclude: 'finish_reasons',
    paramExclude: 'exclude_finish_reason'
  },
  {
    key: 'relay_mode',
    labelKey: 'logPage.relayMode.filterLabel',
    type: 'enum',
    supportsExclude: true,
    options: RELAY_MODES,
    paramInclude: 'relay_modes',
    paramExclude: 'exclude_relay_mode'
  },
  {
    key: 'min_quota',
    labelKey: 'logPage.finishReason.minQuotaLabel',
    placeholderKey: 'logPage.finishReason.minQuotaPlaceholder',
    type: 'number'
  },
  {
    key: 'min_tokens',
    labelKey: 'logPage.finishReason.minTokensLabel',
    placeholderKey: 'logPage.finishReason.minTokensPlaceholder',
    type: 'number'
  }
];

// 通用请求维度筛选。
const commonFields = ({ modelOptions, tokenOptions, appOptions } = {}) => [
  tokenField(tokenOptions),
  modelField(modelOptions),
  appField(appOptions),
  ...staticCommonFields
];

// 组织成员维度:选项来自成员列表(value=user_id),不允许自由输入。
const memberField = (orgMembers) => ({
  key: 'member_id',
  labelKey: 'tableToolBar.memberName',
  type: 'enum',
  supportsExclude: true,
  freeText: false,
  paramInclude: 'member_ids',
  paramExclude: 'exclude_member_id',
  options: (orgMembers || []).map((m) => ({ value: String(m.user_id), label: m.username || `#${m.user_id}` }))
});

// 平台管理员(非组织)维度。channel_id 预置渠道选项(label「名称 #id」, value=id),仍可打字;
// username / source_ip 值域无界,保留文本自由输入不做选项。
const channelField = (channelOptions) =>
  enumField('channel_id', 'tableToolBar.channelId', {
    freeText: true,
    options: (channelOptions || []).map((c) => ({ value: String(c.id), label: c.name ? `${c.name} #${c.id}` : `#${c.id}` }))
  });

const adminFields = (channelOptions) => [
  channelField(channelOptions),
  enumField('username', 'tableToolBar.username', { freeText: true }),
  enumField('source_ip', 'tableToolBar.sourceIp', { freeText: true })
];

export function buildLogFilterFields({
  effectiveAdmin,
  isOrgContext,
  orgMembers,
  modelOptions,
  tokenOptions,
  appOptions,
  channelOptions
} = {}) {
  const fields = [...commonFields({ modelOptions, tokenOptions, appOptions })];
  if (isOrgContext) {
    fields.push(memberField(orgMembers));
  } else {
    // request_id 仅 admin/self 路径有后端绑定(LogsListParams),组织上下文不出现。
    fields.push(requestIdField);
    if (effectiveAdmin) {
      fields.push(...adminFields(channelOptions), upstreamRequestIdField);
    }
  }
  return fields;
}

// 所有上下文可能出现的筛选参数键(URL 同步时先全清再重写,避免切换上下文后残留)。
// 覆盖 number/text 键与 enum 的 include/exclude 键;数值来源于全字段并集。
export const LOG_FILTER_PARAM_KEYS = (() => {
  const all = [...commonFields(), memberField([]), ...adminFields(), requestIdField, upstreamRequestIdField];
  const keys = new Set();
  for (const f of all) {
    if (f.type === 'number' || f.type === 'text') keys.add(f.key);
    else {
      keys.add(f.paramInclude);
      keys.add(f.paramExclude);
    }
  }
  return Array.from(keys);
})();
