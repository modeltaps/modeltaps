import { Braces, Brain, Wrench } from 'lucide-react';

// 模型能力词表（与后端 model_info.capabilities 的 JSON 数组取值一致）。
// 键顺序即展示顺序；文案走 i18n modelpricePage.capability.*，text 仅作兜底。
export const CAPABILITY_OPTIONS = {
  tool_call: {
    value: 'tool_call',
    text: 'Tool Call',
    color: 'info',
    icon: Wrench
  },
  reasoning: {
    value: 'reasoning',
    text: 'Reasoning',
    color: 'warning',
    icon: Brain
  },
  structured_output: {
    value: 'structured_output',
    text: 'Structured Output',
    color: 'success',
    icon: Braces
  }
};
