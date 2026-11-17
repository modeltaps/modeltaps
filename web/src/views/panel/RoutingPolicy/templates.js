// 策略模板：一键填充路由表与问题，目标模型留空由管理员选。文案键在 routingPolicy.templates.*。
// 标签用英文短词，决策模型回答时更稳定；描述是给决策模型看的，也用英文。
export const POLICY_TEMPLATES = [
  {
    // What a coding agent (Claude Code, Codex) is asked to do: reading and answering is cheap,
    // planning a larger piece of work needs the strongest model.
    id: 'agent_task',
    question: 'This is the first thing a developer typed into a coding agent. What do they want the agent to do?',
    routes: [
      { label: 'explain', description: 'Read code or docs and answer a question about them; nothing is edited' },
      { label: 'edit', description: 'Make a concrete, already decided change: add a flag, rename a field, write a test, update a config' },
      { label: 'debug', description: 'Find out why something fails: an error message, a failing test, a crash or wrong output' },
      { label: 'plan', description: 'Design or carry out a larger piece of work over many files: a new feature, a migration, an architecture change' }
    ]
  },
  {
    // One general model name for a team: code goes to a coding model, prose to a writing model,
    // numbers and tables to a model that is good at analysis.
    id: 'content_type',
    question: 'What is the main material of this request?',
    routes: [
      { label: 'chat', description: 'Small talk or a short everyday question that needs no special skill' },
      { label: 'writing', description: 'Drafting, rewriting, translating or summarising text such as an email, an article or a report' },
      { label: 'code', description: 'Source code, shell commands, SQL or configuration files' },
      { label: 'data', description: 'Numbers, tables, spreadsheets or calculations to analyse' }
    ]
  },
  {
    // Requests where a wrong answer is costly go to the most careful model, the rest stay cheap.
    id: 'stakes',
    question: 'How costly would a wrong or careless answer to this request be?',
    routes: [
      { label: 'routine', description: 'Everyday work where a small mistake is easy to notice and fix' },
      { label: 'careful', description: 'Legal, medical, financial or security matters, or anything that will be sent to customers as is' }
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
