// ==============================|| MODEL PRICE — FILTER FIELD DEFINITIONS ||============================== //
// 驱动可复用 FilterBar 的字段定义（可用模型页专属，非组件本身）。各维筛选全部为 enum 多选 + 排除：
//   供应商(owned_by) / 标签(tags) / 输入模态(input_modalities) / 输出模态(output_modalities)
//   / 上下文长度(context) / 价格分桶(price)。
// 与日志页不同：这里 FilterBar 状态仅用于客户端过滤（见 index.jsx 的 useMemo 过滤链），
// 不经 filterStateToParams 转后端参数，故字段无 paramInclude/paramExclude。

import { MODALITY_OPTIONS } from 'constants/Modality';
import { CAPABILITY_OPTIONS } from 'constants/Capability';

// 上下文长度分桶（非重叠区间）：≤4K / 4K–16K / 16K–32K / 32K–128K / 128K–200K / 200K+
export const CONTEXT_BUCKETS = [
  { key: 'le4k', label: '≤4K', match: (c) => c <= 4096 },
  { key: 'le16k', label: '4K–16K', match: (c) => c > 4096 && c <= 16384 },
  { key: 'le32k', label: '16K–32K', match: (c) => c > 16384 && c <= 32768 },
  { key: 'le128k', label: '32K–128K', match: (c) => c > 32768 && c <= 131072 },
  { key: 'le200k', label: '128K–200K', match: (c) => c > 131072 && c <= 200000 },
  { key: 'gt200k', label: '200K+', match: (c) => c > 200000 }
];

// 价格分桶：按所选分组的输入价格（美元 / 每 1M tokens），仅对按量付费(tokens)模型生效。
export const PRICE_BUCKETS = [
  { key: 'free', match: (p) => p === 0 },
  { key: 'le1', label: '≤$1', match: (p) => p > 0 && p <= 1 },
  { key: 'le5', label: '$1–5', match: (p) => p > 1 && p <= 5 },
  { key: 'le15', label: '$5–15', match: (p) => p > 5 && p <= 15 },
  { key: 'gt15', label: '>$15', match: (p) => p > 15 }
];

// 模态候选项：按 MODALITY_OPTIONS 固定顺序保留数据中真实出现的模态，label 走 i18n。
export function buildModalityOptions(t, presentValues = []) {
  const present = new Set(presentValues);
  return Object.keys(MODALITY_OPTIONS)
    .filter((v) => present.has(v))
    .map((v) => ({ value: v, label: t(`modelpricePage.modality.${v}`, { defaultValue: MODALITY_OPTIONS[v].text }) }));
}

// 能力候选项：按 CAPABILITY_OPTIONS 固定顺序保留数据中真实出现的能力，label 走 i18n。
export function buildCapabilityOptions(t, presentValues = []) {
  const present = new Set(presentValues);
  return Object.keys(CAPABILITY_OPTIONS)
    .filter((v) => present.has(v))
    .map((v) => ({ value: v, label: t(`modelpricePage.capability.${v}`, { defaultValue: CAPABILITY_OPTIONS[v].text }) }));
}

// 构建 FilterBar 字段定义。providerOptions/tagOptions/模态候选项由调用方按真实数据装配（不含「全部」，
// 多选语义下「空 = 不约束」）；context/price 直接由分桶派生。price 的 free 走 i18n，其余为字面标签。
export function buildModelFilterFields({
  t,
  providerOptions = [],
  tagOptions = [],
  inputModalityOptions = [],
  outputModalityOptions = [],
  capabilityOptions = []
} = {}) {
  const fields = [
    {
      key: 'owned_by',
      labelKey: 'modelpricePage.channelType',
      type: 'enum',
      supportsExclude: true,
      options: providerOptions
    }
  ];
  // 无可用标签时隐藏标签维度。
  if (tagOptions.length > 0) {
    fields.push({
      key: 'tags',
      labelKey: 'modelpricePage.tags',
      type: 'enum',
      supportsExclude: true,
      options: tagOptions
    });
  }
  // 无对应模态数据时隐藏该维度。
  if (inputModalityOptions.length > 0) {
    fields.push({
      key: 'input_modalities',
      labelKey: 'modelpricePage.inputModality',
      type: 'enum',
      supportsExclude: true,
      options: inputModalityOptions
    });
  }
  if (outputModalityOptions.length > 0) {
    fields.push({
      key: 'output_modalities',
      labelKey: 'modelpricePage.outputModality',
      type: 'enum',
      supportsExclude: true,
      options: outputModalityOptions
    });
  }
  // 无任何能力数据时隐藏该维度。
  if (capabilityOptions.length > 0) {
    fields.push({
      key: 'capabilities',
      labelKey: 'modelpricePage.capabilities',
      type: 'enum',
      supportsExclude: true,
      options: capabilityOptions
    });
  }
  fields.push(
    {
      key: 'context',
      labelKey: 'modelpricePage.contextLength',
      type: 'enum',
      supportsExclude: true,
      options: CONTEXT_BUCKETS.map((b) => ({ value: b.key, label: b.label }))
    },
    {
      key: 'price',
      labelKey: 'modelpricePage.price',
      type: 'enum',
      supportsExclude: true,
      options: PRICE_BUCKETS.map((b) => ({ value: b.key, label: b.key === 'free' ? t('modelpricePage.free') : b.label }))
    }
  );
  return fields;
}
