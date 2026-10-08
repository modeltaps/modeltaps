// ==============================|| PLAYGROUND — CHAT PARAM FIELDS ||============================== //
// 输入框里那颗参数 chip 的字段描述：形状与 shared/fieldSchema 的字段一致（FieldRenderer 直接吃），
// 但键名对齐会话 state（useChatSession 的 settings + system），所以留在对话装配层而不是 shared。
// 数值字段一律「留空 = 不发送」，因此都是 number 而不是 range（滑块没有「空」这个位置）。
// 能力型字段只对声明了对应 capability 的模型出现——多发一个字段会被上游整单拒掉。

const hasCapability = (model, capability) => (model?.info?.capabilities || []).includes(capability);

// reasoning.effort 的取值集合（后端 ChatReasoning.effort 直通上游的 reasoning_effort）。
// 选项里的空串是「默认」：不发 effort，由上游自己决定。
export const REASONING_EFFORTS = ['low', 'medium', 'high'];

export const CHAT_FIELDS = [
  {
    key: 'system',
    group: 'generation',
    type: 'text',
    labelKey: 'playgroundConsole.chat.settings.system',
    placeholderKey: 'playgroundConsole.chat.settings.systemPlaceholder',
    default: ''
  },
  {
    key: 'temperature',
    group: 'generation',
    type: 'number',
    labelKey: 'playgroundConsole.chat.settings.temperature',
    min: 0,
    max: 2,
    step: 0.1,
    default: ''
  },
  {
    key: 'topP',
    group: 'generation',
    type: 'number',
    labelKey: 'playgroundConsole.chat.settings.topP',
    min: 0,
    max: 1,
    step: 0.05,
    default: ''
  },
  {
    key: 'maxTokens',
    group: 'generation',
    type: 'number',
    labelKey: 'playgroundConsole.chat.settings.maxTokens',
    min: 1,
    max: 32768,
    step: 256,
    default: ''
  },
  {
    key: 'seed',
    group: 'generation',
    type: 'number',
    labelKey: 'playgroundConsole.chat.settings.seed',
    default: ''
  },
  {
    key: 'thinking',
    group: 'thinking',
    type: 'switch',
    labelKey: 'playgroundConsole.chat.settings.thinking',
    default: false,
    showWhen: (model) => hasCapability(model, 'reasoning')
  },
  {
    key: 'reasoningEffort',
    group: 'thinking',
    type: 'select',
    labelKey: 'playgroundConsole.chat.settings.reasoningEffort',
    options: ['', ...REASONING_EFFORTS],
    optionLabelKey: 'playgroundConsole.chat.settings.reasoningEffortOptions',
    default: '',
    showWhen: (model, values) => hasCapability(model, 'reasoning') && Boolean(values?.thinking)
  },
  {
    key: 'thinkingBudget',
    group: 'thinking',
    type: 'number',
    labelKey: 'playgroundConsole.chat.settings.thinkingBudget',
    min: 1024,
    max: 32000,
    step: 1024,
    default: '',
    showWhen: (model, values) => hasCapability(model, 'reasoning') && Boolean(values?.thinking)
  },
  {
    key: 'stream',
    group: 'output',
    type: 'switch',
    labelKey: 'playgroundConsole.chat.settings.stream',
    default: true
  },
  {
    key: 'jsonMode',
    group: 'output',
    type: 'switch',
    labelKey: 'playgroundConsole.chat.settings.jsonMode',
    default: false,
    showWhen: (model) => hasCapability(model, 'structured_output')
  }
];

// chip 上那行摘要：只报改过的关键值（温度 · 最大 tokens），一个都没改就返回空串，
// 由调用方回落成「参数」二字——chip 上不写「默认 · 默认」这种没有信息量的字样。
export function paramsSummary(settings = {}) {
  const parts = [];
  for (const key of ['temperature', 'maxTokens']) {
    const value = settings[key];
    if (value === '' || value === null || value === undefined) continue;
    parts.push(String(value));
  }
  return parts.join(' · ');
}
