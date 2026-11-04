// 策略模板：一键填充路由表与问题，目标模型留空由管理员选。文案键在 routingPolicy.templates.*。
// 标签用英文短词，决策模型回答时更稳定；描述是给决策模型看的，也用英文。
export const POLICY_TEMPLATES = [
  {
    id: 'coding_change_size',
    question: 'The user is asking a coding agent for help. How large is the change they are asking for?',
    routes: [
      {
        label: 'additive',
        description: 'Adds something new without changing existing behaviour: a log line, a flag, a test, a small helper'
      },
      { label: 'single_change', description: 'Changes existing behaviour in one place: fix a bug, adjust one function or component' },
      { label: 'multi_change', description: 'Changes behaviour across several files or modules, or touches shared interfaces' },
      { label: 'refactor', description: 'Restructures code without changing behaviour, or asks for design and architecture work' }
    ]
  },
  {
    id: 'support_triage',
    question: 'This is the first message of a customer support conversation. What kind of request is it?',
    routes: [
      { label: 'faq', description: 'A common question answered by documentation: pricing, hours, how to do a basic task' },
      { label: 'account', description: 'Account or billing issue that needs looking up records or changing settings' },
      { label: 'technical', description: 'A bug report or technical problem that needs diagnosis' },
      { label: 'escalation', description: 'Complaint, legal or safety matter, or an angry customer that needs careful handling' }
    ]
  },
  {
    id: 'reasoning_depth',
    question: 'How much reasoning does answering this request need?',
    routes: [
      { label: 'lookup', description: 'A factual question, a definition, a translation or a short rewrite' },
      { label: 'moderate', description: 'Needs some analysis or a few steps: summarising, comparing, writing a structured piece' },
      { label: 'deep', description: 'Needs careful multi-step reasoning: maths, complex planning, tricky debugging, nuanced judgement' }
    ]
  }
];

// applyTemplate 把模板合并进表单：问题与路由表替换，已有目标按位置保留。
export function applyTemplate(form, template) {
  const routes = template.routes.map((route, index) => ({
    ...route,
    target_model: form.routes?.[index]?.target_model || ''
  }));
  return { ...form, question: template.question, routes };
}
